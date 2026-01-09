// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/luxfi/cache/lru"
	"github.com/luxfi/cache/metercacher"
	"github.com/luxfi/codec"
	"github.com/luxfi/consensus"
	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/consensus/engine/chain/block"
	consensusversion "github.com/luxfi/consensus/version"
	"github.com/luxfi/constantsants"
	"github.com/luxfi/database"
	"github.com/luxfi/database/versiondb"
	"github.com/luxfi/ids"
	"github.com/luxfi/log"
	"github.com/luxfi/metric"
	"github.com/luxfi/p2p"
	p2pgossip "github.com/luxfi/p2p/gossip"
	"github.com/luxfi/sdk/utils"
	"github.com/luxfi/sdk/utils/perms"
	"github.com/luxfi/sdk/utils/profiler"
	"github.com/luxfi/vm"
	"github.com/luxfi/vm/chain"
	"github.com/luxfi/vm/utils/timer/mockable"
	"github.com/luxfi/vm/vms/components/gas"
	luxwarp "github.com/luxfi/warp"

	"github.com/luxfi/coreth/consensus/dummy"
	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/txpool"
	"github.com/luxfi/coreth/eth"
	"github.com/luxfi/coreth/eth/ethconfig"
	corethprometheus "github.com/luxfi/coreth/metrics/prometheus"
	"github.com/luxfi/coreth/miner"
	"github.com/luxfi/coreth/network"
	"github.com/luxfi/coreth/node"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/config"
	"github.com/luxfi/coreth/plugin/evm/customrawdb"
	"github.com/luxfi/coreth/plugin/evm/extension"
	"github.com/luxfi/coreth/plugin/evm/gossip"
	corethlog "github.com/luxfi/coreth/plugin/evm/log"
	"github.com/luxfi/coreth/plugin/evm/message"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/coreth/plugin/evm/vmerrors"
	warpcontract "github.com/luxfi/coreth/precompile/contracts/warp"
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/coreth/rpc"
	statesyncclient "github.com/luxfi/coreth/sync/client"
	"github.com/luxfi/coreth/sync/client/stats"
	"github.com/luxfi/coreth/sync/handlers"
	handlerstats "github.com/luxfi/coreth/sync/handlers/stats"
	vmsync "github.com/luxfi/coreth/sync/vm"
	utilsrpc "github.com/luxfi/coreth/utils/rpc"
	"github.com/luxfi/coreth/warp"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/metrics"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/geth/triedb"
	"github.com/luxfi/geth/triedb/hashdb"

	// Force-load tracer engine to trigger registration
	//
	// We must import this package (not referenced elsewhere) so that the native "callTracer"
	// is added to a map of client-accessible tracers. In geth, this is done
	// inside of cmd/geth.
	_ "github.com/luxfi/geth/eth/tracers/js"
	_ "github.com/luxfi/geth/eth/tracers/native"

	// Force-load precompiles to trigger registration
	_ "github.com/luxfi/coreth/precompile/registry"
)

var (
	// Interface compliance checks removed - EVM doesn't implement network interfaces
	_ statesyncclient.EthBlockParser = (*VM)(nil)
	_ vmsync.BlockAcceptor           = (*VM)(nil)
)

const (
	// Max time from current time allowed for blocks, before they're considered future blocks
	// and fail verification
	maxFutureBlockTime = 10 * time.Second

	secpCacheSize          = 1024
	decidedCacheSize       = 10 * constants.MiB
	missingCacheSize       = 50
	unverifiedCacheSize    = 5 * constants.MiB
	bytesToIDCacheSize     = 5 * constants.MiB
	warpSignatureCacheSize = 500

	// Prefixes for metrics gatherers
	ethMetricsPrefix        = "eth"
	sdkMetricsPrefix        = "sdk"
	chainStateMetricsPrefix = "chain_state"
)

// Define the API endpoints for the VM
const (
	adminEndpoint        = "/admin"
	ethRPCEndpoint       = "/rpc"
	ethWSEndpoint        = "/ws"
	ethTxGossipNamespace = "eth_tx_gossip"
)

var (
	// Set last accepted key to be longer than the keys used to store accepted block IDs.
	lastAcceptedKey = []byte("last_accepted_key")
	acceptedPrefix  = []byte("quasarman_accepted")
	metadataPrefix  = []byte("metadata")
	warpPrefix      = []byte("warp")
	ethDBPrefix     = []byte("ethdb")
)

var (
	errInvalidBlock                  = errors.New("invalid block")
	errInvalidNonce                  = errors.New("invalid nonce")
	errUnclesUnsupported             = errors.New("uncles unsupported")
	errNilBaseFeeApricotPhase3       = errors.New("nil base fee is invalid after apricotPhase3")
	errNilBlockGasCostApricotPhase4  = errors.New("nil blockGasCost is invalid after apricotPhase4")
	errInvalidHeaderPredicateResults = errors.New("invalid header predicate results")
	errInitializingLogger            = errors.New("failed to initialize logger")
	errShuttingDownVM                = errors.New("shutting down VM")
)

var originalStderr *os.File

// legacyApiNames maps pre geth v1.10.20 api names to their updated counterparts.
// used in attachEthService for backward configuration compatibility.
var legacyApiNames = map[string]string{
	"internal-public-eth":              "internal-eth",
	"internal-public-blockchain":       "internal-blockchain",
	"internal-public-transaction-pool": "internal-transaction",
	"internal-public-tx-pool":          "internal-tx-pool",
	"internal-public-debug":            "internal-debug",
	"internal-private-debug":           "internal-debug",
	"internal-public-account":          "internal-account",
	"internal-private-personal":        "internal-personal",

	"public-eth":        "eth",
	"public-eth-filter": "eth-filter",
	"private-admin":     "admin",
	"public-debug":      "debug",
	"private-debug":     "debug",
}

func init() {
	// Preserve [os.Stderr] prior to the call in plugin/main.go to plugin.Serve(...).
	// Preserving the log level allows us to update the root handler while writing to the original
	// [os.Stderr] that is being piped through to the logger via the rpcchainv.
	originalStderr = os.Stderr
}

// VM implements the quasarman.ChainVM interface
type VM struct {
	ctx *consensusctx.Context
	// [cancel] may be nil until [quasar.NormalOp] starts
	cancel context.CancelFunc
	// *chain.State helps to implement the VM interface by wrapping blocks
	// with an efficient caching layer.
	*chain.State

	config config.Config

	chainID     *big.Int
	genesisHash common.Hash
	chainConfig *params.ChainConfig
	ethConfig   ethconfig.Config

	// Extension Points
	extensionConfig *extension.Config

	// pointers to eth constructs
	eth        *eth.Ethereum
	txPool     *txpool.TxPool
	blockChain *core.BlockChain
	miner      *miner.Miner

	// [versiondb] is the VM's current versioned database
	versiondb *versiondb.Database

	// [db] is the VM's current database
	db database.Database

	// metadataDB is used to store one off keys.
	metadataDB database.Database

	// [chaindb] is the database supplied to the Ethereum backend
	chaindb ethdb.Database

	// [acceptedBlockDB] is the database to store the last accepted
	// block.
	acceptedBlockDB database.Database

	// [warpDB] is used to store warp message signatures
	// set to a prefixDB with the prefix [warpPrefix]
	warpDB database.Database

	// builderLock is used to synchronize access to the block builder,
	// as it is uninitialized at first and is only initialized when onNormalOperationsStarted is called.
	builderLock sync.Mutex
	builder     *blockBuilder

	clock *mockable.Clock

	shutdownChan chan struct{}
	shutdownWg   sync.WaitGroup

	// Continuous Profiler
	profiler profiler.ContinuousProfiler

	network.Network
	networkCodec codec.Manager

	// Metrics
	sdkMetrics      *prometheus.Registry
	metricsGatherer metric.MultiGatherer // Type-asserted from ctx.Metrics

	// Type-asserted context interfaces
	ctxLogger log.Logger // Type-asserted from ctx.Log

	bootstrapped utils.Atomic[bool]
	IsPlugin     bool

	// toEngine is used to notify the consensus engine about pending transactions
	// Uses block.Message to match the exact channel type from chains/manager.go
	toEngine chan<- block.Message

	stateSyncDone chan struct{}

	logger corethlog.Logger
	// State sync server and client
	vmsync.Server
	vmsync.Client

	// Lux Warp Messaging backend
	// Used to serve BLS signatures of warp messages over RPC
	warpBackend warp.Backend

	// Initialize only sets these if nil so they can be overridden in tests
	ethTxGossipHandler p2p.Handler
	ethTxPushGossiper  utils.Atomic[*p2pgossip.PushGossiper[*GossipEthTx]]
	ethTxPullGossiper  p2pgossip.Gossiper

	chainAlias string
	// RPC handlers (should be stopped before closing chaindb)
	rpcHandlers []interface{ Stop() }
}

// Initialize implements the quasarman.ChainVM interface
func (v *VM) Initialize(
	_ context.Context,
	chainCtx interface{},
	db interface{},
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine interface{},
	_ []interface{},
	appSenderIface interface{},
) error {
	// Type assert appSender - node provides p2p.Sender
	var appSender network.AppSender
	if appSenderIface != nil {
		if p2pSender, ok := appSenderIface.(p2p.Sender); ok {
			appSender = network.NewP2PSenderAdapter(p2pSender)
		} else if networkSender, ok := appSenderIface.(network.AppSender); ok {
			appSender = networkSender
		} else {
			return fmt.Errorf("expected p2p.Sender or network.AppSender, got %T", appSenderIface)
		}
	}
	// Type assert chainCtx
	vmCtx, ok := chainCtx.(*consensusctx.Context)
	if !ok {
		return fmt.Errorf("expected *consensusctx.Context, got %T", chainCtx)
	}
	// Type assert database
	vmDB, ok := db.(database.Database)
	if !ok {
		return fmt.Errorf("expected database.Database, got %T", db)
	}
	if err := v.extensionConfig.Validate(); err != nil {
		return fmt.Errorf("failed to validate extension config: %w", err)
	}

	v.clock = v.extensionConfig.Clock

	v.config.SetDefaults(defaultTxPoolConfig)
	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &v.config); err != nil {
			return fmt.Errorf("failed to unmarshal config %s: %w", string(configBytes), err)
		}
	}
	v.ctx = vmCtx

	// Type assert and store context interfaces for use throughout VM
	if ctxLogger, ok := v.ctx.Log.(log.Logger); ok {
		v.ctxLogger = ctxLogger
	} else {
		return fmt.Errorf("expected log.Logger for ctx.Log, got %T", v.ctx.Log)
	}
	if metricsGatherer, ok := v.ctx.Metrics.(metric.MultiGatherer); ok {
		v.metricsGatherer = metricsGatherer
	} else {
		return fmt.Errorf("expected metric.MultiGatherer for ctx.Metrics, got %T", v.ctx.Metrics)
	}

	if err := v.config.Validate(v.ctx.NetworkID); err != nil {
		return err
	}
	// We should deprecate config flags as the first thing, before we do anything else
	// because this can set old flags to new flags. log the message after we have
	// initialized the logger.
	deprecateMsg := v.config.Deprecate()

	// Get chain alias from BCLookup with defensive nil check
	var alias string
	if v.ctx.BCLookup != nil {
		var err error
		alias, err = v.ctx.BCLookup.PrimaryAlias(v.ctx.ChainID)
		if err != nil {
			// fallback to ChainID string instead of erroring
			alias = v.ctx.ChainID.String()
		}
	} else {
		// BCLookup is nil - use ChainID string as fallback
		// This can happen with plugin VMs if context is not properly propagated
		alias = v.ctx.ChainID.String()
	}
	v.chainAlias = alias

	var writer io.Writer = originalStderr
	if logWriter, ok := v.ctx.Log.(io.Writer); ok && !v.IsPlugin {
		writer = logWriter
	}

	corethLogger, err := corethlog.InitLogger(v.chainAlias, v.config.LogLevel, v.config.LogJSONFormat, writer)
	if err != nil {
		return fmt.Errorf("%w: %w ", errInitializingLogger, err)
	}
	v.logger = corethLogger

	log.Info("Initializing Coreth VM", "Version", Version, "Config", v.config)
	log.Info("CORETH_BUILD_2026_01_01_1605", "marker", "PLUGIN_LOADED_FROM_PLUGINS_DIR")
	log.Info("DEBUG upgradeBytes received", "length", len(upgradeBytes), "content", string(upgradeBytes))

	// Store toEngine channel for notifying consensus about pending transactions
	// The manager passes chan block.Message (bidirectional), we store it as send-only.
	// IMPORTANT: Must use block.Message type exactly to match chains/manager.go channel type.
	log.Info("toEngine dynamic type", "type", fmt.Sprintf("%T", toEngine))
	switch ch := toEngine.(type) {
	case chan<- block.Message:
		v.toEngine = ch
		log.Info("toEngine stored", "as", "chan<- block.Message")
	case chan block.Message:
		// Cast bidirectional channel to send-only
		v.toEngine = (chan<- block.Message)(ch)
		log.Info("toEngine stored", "as", "chan block.Message (cast to send-only)")
	default:
		if toEngine == nil {
			log.Warn("toEngine is nil - cannot notify consensus on PendingTxs")
		} else {
			log.Warn("unexpected toEngine type", "type", fmt.Sprintf("%T", toEngine))
		}
	}

	if deprecateMsg != "" {
		log.Warn("Deprecation Warning", "msg", deprecateMsg)
	}

	// Enable debug-level metrics that might impact runtime performance
	// Note: MetricsExpensiveEnabled config is stored but expensive metrics
	// are handled internally by luxfi/metric package
	_ = v.config.MetricsExpensiveEnabled // Keep config but don't use geth metrics

	v.shutdownChan = make(chan struct{}, 1)

	if err := v.initializeMetrics(); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Initialize the database
	if err := v.initializeDBs(vmDB); err != nil {
		return fmt.Errorf("failed to initialize databases: %w", err)
	}

	if v.config.InspectDatabase {
		if err := v.inspectDatabases(); err != nil {
			return err
		}
	}

	// Handle chain data import if configured
	if v.config.ImportChainData != "" {
		log.Info("Importing chain data", "path", v.config.ImportChainData)
		if err := v.importChainData(v.config.ImportChainData); err != nil {
			return fmt.Errorf("failed to import chain data: %w", err)
		}
	}

	g, err := parseGenesis(vmCtx, genesisBytes)
	if err != nil {
		return err
	}

	// Parse and apply upgrade configuration from upgradeBytes (upgrade.json)
	if len(upgradeBytes) > 0 {
		var upgradeConfig extras.UpgradeConfig
		if err := json.Unmarshal(upgradeBytes, &upgradeConfig); err != nil {
			return fmt.Errorf("failed to parse upgrade config: %w", err)
		}

		// Merge precompile upgrades from upgrade.json into chain config
		if len(upgradeConfig.PrecompileUpgrades) > 0 {
			configExtra := params.GetExtra(g.Config)
			configExtra.PrecompileUpgrades = append(configExtra.PrecompileUpgrades, upgradeConfig.PrecompileUpgrades...)
			log.Info("Applied precompile upgrades from upgrade.json",
				"count", len(upgradeConfig.PrecompileUpgrades))
		}
	}

	// v.ChainConfig() should be available for wrapping VMs before v.initializeChain()
	v.chainConfig = g.Config
	v.chainID = g.Config.ChainID

	v.ethConfig = ethconfig.NewDefaultConfig()
	v.ethConfig.Genesis = g
	v.ethConfig.NetworkId = v.chainID.Uint64()
	// Genesis hash is computed by geth's Header.Hash() which uses 16-field format
	// for genesis blocks (Number == 0) to match original Lux mainnet genesis hash.
	v.genesisHash = v.ethConfig.Genesis.ToBlock().Hash() // must create genesis hash before [v.ReadLastAccepted]
	lastAcceptedHash, lastAcceptedHeight, err := v.ReadLastAccepted()
	if err != nil {
		return err
	}
	log.Info("read last accepted",
		"hash", lastAcceptedHash,
		"height", lastAcceptedHeight,
	)

	// Set minimum price for mining and default gas price oracle value to the min
	// gas price to prevent so transactions and blocks all use the correct fees
	v.ethConfig.RPCGasCap = v.config.RPCGasCap
	v.ethConfig.RPCEVMTimeout = v.config.APIMaxDuration.Duration
	v.ethConfig.RPCTxFeeCap = v.config.RPCTxFeeCap
	v.ethConfig.PriceOptionConfig.SlowFeePercentage = v.config.PriceOptionSlowFeePercentage
	v.ethConfig.PriceOptionConfig.FastFeePercentage = v.config.PriceOptionFastFeePercentage
	v.ethConfig.PriceOptionConfig.MaxTip = v.config.PriceOptionMaxTip

	v.ethConfig.TxPool.NoLocals = !v.config.LocalTxsEnabled
	v.ethConfig.TxPool.PriceLimit = v.config.TxPoolPriceLimit
	v.ethConfig.TxPool.PriceBump = v.config.TxPoolPriceBump
	v.ethConfig.TxPool.AccountSlots = v.config.TxPoolAccountSlots
	v.ethConfig.TxPool.GlobalSlots = v.config.TxPoolGlobalSlots
	v.ethConfig.TxPool.AccountQueue = v.config.TxPoolAccountQueue
	v.ethConfig.TxPool.GlobalQueue = v.config.TxPoolGlobalQueue
	v.ethConfig.TxPool.Lifetime = v.config.TxPoolLifetime.Duration

	v.ethConfig.AllowUnfinalizedQueries = v.config.AllowUnfinalizedQueries
	v.ethConfig.AllowUnprotectedTxs = v.config.AllowUnprotectedTxs
	v.ethConfig.AllowUnprotectedTxHashes = v.config.AllowUnprotectedTxHashes
	v.ethConfig.Preimages = v.config.Preimages
	v.ethConfig.Pruning = v.config.Pruning
	v.ethConfig.TrieCleanCache = v.config.TrieCleanCache
	v.ethConfig.TrieDirtyCache = v.config.TrieDirtyCache
	v.ethConfig.TrieDirtyCommitTarget = v.config.TrieDirtyCommitTarget
	v.ethConfig.TriePrefetcherParallelism = v.config.TriePrefetcherParallelism
	v.ethConfig.SnapshotCache = v.config.SnapshotCache
	v.ethConfig.AcceptorQueueLimit = v.config.AcceptorQueueLimit
	v.ethConfig.PopulateMissingTries = v.config.PopulateMissingTries
	v.ethConfig.PopulateMissingTriesParallelism = v.config.PopulateMissingTriesParallelism
	v.ethConfig.AllowMissingTries = v.config.AllowMissingTries
	v.ethConfig.SnapshotDelayInit = v.stateSyncEnabled(lastAcceptedHeight)
	v.ethConfig.SnapshotWait = v.config.SnapshotWait
	v.ethConfig.SnapshotVerify = v.config.SnapshotVerify
	v.ethConfig.HistoricalProofQueryWindow = v.config.HistoricalProofQueryWindow
	v.ethConfig.OfflinePruning = v.config.OfflinePruning
	v.ethConfig.OfflinePruningBloomFilterSize = v.config.OfflinePruningBloomFilterSize
	v.ethConfig.OfflinePruningDataDirectory = v.config.OfflinePruningDataDirectory
	v.ethConfig.CommitInterval = v.config.CommitInterval
	v.ethConfig.SkipUpgradeCheck = v.config.SkipUpgradeCheck
	v.ethConfig.AcceptedCacheSize = v.config.AcceptedCacheSize
	v.ethConfig.StateHistory = v.config.StateHistory
	v.ethConfig.TransactionHistory = v.config.TransactionHistory
	v.ethConfig.SkipTxIndexing = v.config.SkipTxIndexing
	v.ethConfig.StateScheme = v.config.StateScheme

	if v.ethConfig.StateScheme == customrawdb.DatabaseScheme {
		log.Warn("Database state scheme is enabled")
		log.Warn("This is untested in production, use at your own risk")
		// Database only supports pruning for now.
		if !v.config.Pruning {
			return errors.New("Pruning must be enabled for Database")
		}
		// Database does not support iterators, so the snapshot cannot be constructed
		if v.config.SnapshotCache > 0 {
			return errors.New("Snapshot cache must be disabled for Database")
		}
		if v.config.OfflinePruning {
			return errors.New("Offline pruning is not supported for Database")
		}
		if v.config.StateSyncEnabled == nil || *v.config.StateSyncEnabled {
			return errors.New("State sync is not yet supported for Database")
		}
	}
	if v.ethConfig.StateScheme == rawdb.PathScheme {
		log.Error("Path state scheme is not supported. Please use HashDB or Database state schemes instead")
		return errors.New("Path state scheme is not supported")
	}

	// Create directory for offline pruning
	if len(v.ethConfig.OfflinePruningDataDirectory) != 0 {
		if err := os.MkdirAll(v.ethConfig.OfflinePruningDataDirectory, perms.ReadWriteExecute); err != nil {
			log.Error("failed to create offline pruning data directory", "error", err)
			return err
		}
	}

	v.chainConfig = g.Config

	v.networkCodec = message.Codec
	v.Network, err = network.NewNetwork(v.ctx, appSender, v.networkCodec, v.config.MaxOutboundActiveRequests, v.sdkMetrics)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Initialize warp backend
	offchainWarpMessages := make([][]byte, len(v.config.WarpOffChainMessages))
	for i, hexMsg := range v.config.WarpOffChainMessages {
		offchainWarpMessages[i] = []byte(hexMsg)
	}
	warpSignatureCache := lru.NewCache[ids.ID, []byte](warpSignatureCacheSize)
	meteredCache, err := metercacher.New("warp_signature_cache", v.sdkMetrics, warpSignatureCache)
	if err != nil {
		return fmt.Errorf("failed to create warp signature cache: %w", err)
	}

	// clear warpdb on initialization if config enabled
	if v.config.PruneWarpDB {
		if err := database.Clear(v.warpDB, ethdb.IdealBatchSize); err != nil {
			return fmt.Errorf("failed to prune warpDB: %w", err)
		}
	}

	// Type assert WarpSigner from context - may be nil for plugin VMs
	if v.ctx.WarpSigner != nil {
		warpSigner, ok := v.ctx.WarpSigner.(luxwarp.Signer)
		if !ok {
			return fmt.Errorf("expected luxwarp.Signer, got %T", v.ctx.WarpSigner)
		}

		v.warpBackend, err = warp.NewBackend(
			v.ctx.NetworkID,
			v.ctx.ChainID,
			warpSigner,
			v,
			v.warpDB,
			meteredCache,
			offchainWarpMessages,
		)
		if err != nil {
			return err
		}
	} else {
		log.Info("WarpSigner is nil, warp messaging disabled for this chain")
	}

	if err := v.initializeChain(lastAcceptedHash); err != nil {
		return err
	}

	// Handle RLP import after blockchain is initialized
	if v.config.ImportChainData != "" && strings.HasSuffix(strings.ToLower(v.config.ImportChainData), ".rlp") {
		if err := v.importRLPBlocks(v.config.ImportChainData); err != nil {
			return fmt.Errorf("failed to import RLP blocks: %w", err)
		}
	}

	// Use stored ctxLogger for RecoverAndPanic
	go v.ctxLogger.RecoverAndPanic(v.startContinuousProfiler)

	// Add p2p warp message handler if WarpSigner is available
	if v.ctx.WarpSigner != nil {
		warpSigner, ok := v.ctx.WarpSigner.(luxwarp.Signer)
		if !ok {
			return fmt.Errorf("expected warp.Signer, got %T", v.ctx.WarpSigner)
		}

		// Add p2p warp message handler
		warpHandler := luxwarp.NewCachedSignatureHandler(meteredCache, v.warpBackend, warpSigner)
		v.Network.AddHandler(p2p.SignatureRequestHandlerID, luxwarp.NewSignatureHandlerAdapter(warpHandler))
	}

	v.stateSyncDone = make(chan struct{})

	return v.initializeStateSync(lastAcceptedHeight)
}

func parseGenesis(ctx *consensusctx.Context, bytes []byte) (*core.Genesis, error) {
	g := new(core.Genesis)
	if err := json.Unmarshal(bytes, g); err != nil {
		return nil, fmt.Errorf("parsing genesis: %w", err)
	}

	// Populate the Lux config extras.
	configExtra := params.GetExtra(g.Config)
	configExtra.LuxContext = extras.LuxContext{
		ConsensusCtx: ctx,
	}

	// Parse the genesis config into extras to get Lux upgrade timestamps
	// The geth ChainConfig doesn't have these fields, so we need to parse separately
	type genesisConfig struct {
		Config *extras.ChainConfig `json:"config"`
	}
	var gc genesisConfig
	if err := json.Unmarshal(bytes, &gc); err != nil {
		return nil, fmt.Errorf("parsing genesis config extras: %w", err)
	}

	// Use NetworkUpgrades from genesis config if present
	if gc.Config != nil {
		configExtra.NetworkUpgrades = gc.Config.NetworkUpgrades
	}

	// If Durango is scheduled, schedule the Warp Precompile at the same time.
	if configExtra.DurangoBlockTimestamp != nil {
		configExtra.PrecompileUpgrades = append(configExtra.PrecompileUpgrades, extras.PrecompileUpgrade{
			Config: warpcontract.NewDefaultConfig(configExtra.DurangoBlockTimestamp),
		})
	}
	if err := configExtra.Verify(); err != nil {
		return nil, fmt.Errorf("invalid chain config: %w", err)
	}

	// Align all the Ethereum upgrades to the Lux upgrades
	if err := params.SetEthUpgrades(g.Config); err != nil {
		return nil, fmt.Errorf("setting eth upgrades: %w", err)
	}
	return g, nil
}

func (v *VM) initializeMetrics() error {
	metrics.Enable()
	v.sdkMetrics = prometheus.NewRegistry()
	gatherer := corethprometheus.NewGatherer(metrics.DefaultRegistry)

	if err := v.metricsGatherer.Register(ethMetricsPrefix, gatherer); err != nil {
		return err
	}

	// if v.config.MetricsExpensiveEnabled && v.config.StateScheme == customrawdb.DatabaseScheme {
	// 	if err := ffi.StartMetrics(); err != nil {
	// 		return fmt.Errorf("failed to start database metrics collection: %w", err)
	// 	}
	// 	if err := v.metricsGatherer.Register("database", ffi.Gatherer{}); err != nil {
	// 		return fmt.Errorf("failed to register database metrics: %w", err)
	// 	}
	// }
	return v.metricsGatherer.Register(sdkMetricsPrefix, v.sdkMetrics)
}

func (v *VM) initializeChain(lastAcceptedHash common.Hash) error {
	nodecfg := &node.Config{
		CorethVersion:         Version,
		KeyStoreDir:           v.config.KeystoreDirectory,
		ExternalSigner:        v.config.KeystoreExternalSigner,
		InsecureUnlockAllowed: v.config.KeystoreInsecureUnlockAllowed,
	}
	node, err := node.New(nodecfg)
	if err != nil {
		return err
	}

	// If the gas target is specified, calculate the desired target excess and
	// use it during block creation.
	var desiredTargetExcess *gas.Gas
	if v.config.GasTarget != nil {
		desiredTargetExcess = new(gas.Gas)
		*desiredTargetExcess = lp176.DesiredTargetExcess(*v.config.GasTarget)
	}

	v.eth, err = eth.New(
		node,
		&v.ethConfig,
		&EthPushGossiper{vm: v},
		v.chaindb,
		eth.Settings{MaxBlocksPerRequest: v.config.MaxBlocksPerRequest},
		lastAcceptedHash,
		dummy.NewDummyEngine(
			v.extensionConfig.ConsensusCallbacks,
			dummy.Mode{ModeSkipBlockFee: v.config.SkipBlockFee},
			v.clock,
			desiredTargetExcess,
		),
		v.clock,
	)
	if err != nil {
		return err
	}
	v.eth.SetEtherbase(constants.BlackholeAddr)
	v.txPool = v.eth.TxPool()
	v.blockChain = v.eth.BlockChain()
	v.miner = v.eth.Miner()

	// Register callback for admin_importChain to update both:
	// 1. acceptedBlockDB - persistence for ReadLastAccepted() on restart
	// 2. chain.State.lastAcceptedBlock - consensus layer's view of the chain head
	// This is critical: without both updates, imported blocks won't be recognized
	// as the canonical chain head and new blocks cannot be produced.
	v.eth.SetPostImportCallback(func(lastBlockHash common.Hash, lastBlockHeight uint64) error {
		log.Info("PostImportCallback: updating acceptedBlockDB and chain.State", "hash", lastBlockHash, "height", lastBlockHeight)

		// Step 1: Update acceptedBlockDB for persistence
		if err := v.acceptedBlockDB.Put(lastAcceptedKey, lastBlockHash[:]); err != nil {
			return fmt.Errorf("failed to update acceptedBlockDB: %w", err)
		}
		if err := v.versiondb.Commit(); err != nil {
			return fmt.Errorf("failed to commit versiondb: %w", err)
		}
		// CRITICAL: Force sync to disk to ensure persistence across restarts.
		// Without this, async writes (e.g., badgerdb with SyncWrites=false) may be
		// lost if the network stops before the background flush completes.
		if err := v.versiondb.Sync(); err != nil {
			log.Warn("PostImportCallback: sync failed (non-fatal)", "error", err)
			// Don't return error - the commit succeeded, sync is best-effort
		}
		log.Info("PostImportCallback: acceptedBlockDB updated and synced successfully")

		// Step 2: Update chain.State.lastAcceptedBlock for consensus
		// This ensures the Snowman consensus engine recognizes the imported blocks
		// as the canonical chain head.
		if v.State == nil {
			log.Warn("PostImportCallback: chain.State not initialized yet, skipping state update")
			return nil
		}

		// Get the eth block from the blockchain
		ethBlock := v.blockChain.GetBlockByHash(lastBlockHash)
		if ethBlock == nil {
			return fmt.Errorf("failed to get block by hash %s from blockchain", lastBlockHash.Hex())
		}

		// Wrap the eth block as a consensus block
		wrappedBlock, err := wrapBlock(ethBlock, v)
		if err != nil {
			return fmt.Errorf("failed to wrap block: %w", err)
		}

		// Update the chain.State's lastAcceptedBlock
		if err := v.SetLastAcceptedBlock(wrappedBlock); err != nil {
			return fmt.Errorf("failed to set last accepted block in chain.State: %w", err)
		}

		log.Info("PostImportCallback: chain.State updated successfully", "height", lastBlockHeight, "hash", lastBlockHash.Hex())

		// Notify consensus engine to start building blocks from the imported tip
		if v.toEngine != nil {
			select {
			case v.toEngine <- block.Message{Type: block.PendingTxs}:
				log.Info("PostImportCallback: notified consensus engine to resume block production")
			default:
				log.Warn("PostImportCallback: toEngine channel full, consensus notification dropped")
			}
		}
		return nil
	})

	// Set the gas parameters for the tx pool to the minimum gas price for the
	// latest upgrade.
	v.txPool.SetGasTip(big.NewInt(0))
	log.Warn("CORETH-ZACH: SkipBlockFee config value", "skipBlockFee", v.config.SkipBlockFee)
	if v.config.SkipBlockFee {
		// Allow zero-fee transactions when skip-block-fee is enabled
		log.Warn("CORETH-ZACH: Setting MinFee to 0")
		v.txPool.SetMinFee(big.NewInt(0))
	} else {
		log.Warn("CORETH-ZACH: Setting MinFee to lp176.MinGasPrice", "minGasPrice", lp176.MinGasPrice)
		v.txPool.SetMinFee(big.NewInt(lp176.MinGasPrice))
	}

	v.eth.Start()
	return v.initChainState(v.blockChain.LastAcceptedBlock())
}

// initializeStateSync initializes the vm for performing state sync and responding to peer requests.
// If state sync is disabled, this function will wipe any ongoing summary from
// disk to ensure that we do not continue syncing from an invalid snapshot.
func (v *VM) initializeStateSync(lastAcceptedHeight uint64) error {
	// Create standalone EVM TrieDB (read only) for serving leafs requests.
	// We create a standalone TrieDB here, so that it has a standalone cache from the one
	// used by the node when processing blocks.
	// However, Database does not support multiple TrieDBs, so we use the same one.
	evmTrieDB := v.eth.BlockChain().TrieDB()
	if v.ethConfig.StateScheme != customrawdb.DatabaseScheme {
		evmTrieDB = triedb.NewDatabase(
			v.chaindb,
			&triedb.Config{
				HashDB: &hashdb.Config{
					CleanCacheSize: v.config.StateSyncServerTrieCache * constants.MiB,
				},
			},
		)
	}
	leafHandlers := make(LeafHandlers)
	leafMetricsNames := make(map[message.NodeType]string)
	// register default leaf request handler for state trie
	syncStats := handlerstats.GetOrRegisterHandlerStats(metrics.Enabled())
	stateLeafRequestConfig := &extension.LeafRequestConfig{
		LeafType:   message.StateTrieNode,
		MetricName: "sync_state_trie_leaves",
		Handler: handlers.NewLeafsRequestHandler(evmTrieDB,
			message.StateTrieKeyLength,
			v.blockChain, v.networkCodec,
			syncStats,
		),
	}
	leafHandlers[stateLeafRequestConfig.LeafType] = stateLeafRequestConfig.Handler
	leafMetricsNames[stateLeafRequestConfig.LeafType] = stateLeafRequestConfig.MetricName

	extraLeafConfig := v.extensionConfig.ExtraSyncLeafHandlerConfig
	if extraLeafConfig != nil {
		leafHandlers[extraLeafConfig.LeafType] = extraLeafConfig.Handler
		leafMetricsNames[extraLeafConfig.LeafType] = extraLeafConfig.MetricName
	}

	networkHandler := newNetworkHandler(
		v.blockChain,
		v.chaindb,
		v.warpBackend,
		v.networkCodec,
		leafHandlers,
		syncStats,
	)
	v.Network.SetRequestHandler(networkHandler)

	v.Server = vmsync.NewServer(v.blockChain, v.extensionConfig.SyncSummaryProvider, v.config.StateSyncCommitInterval)
	stateSyncEnabled := v.stateSyncEnabled(lastAcceptedHeight)
	// parse nodeIDs from state sync IDs in vm config
	var stateSyncIDs []ids.NodeID
	if stateSyncEnabled && len(v.config.StateSyncIDs) > 0 {
		nodeIDs := strings.Split(v.config.StateSyncIDs, ",")
		stateSyncIDs = make([]ids.NodeID, len(nodeIDs))
		for i, nodeIDString := range nodeIDs {
			nodeID, err := ids.NodeIDFromString(nodeIDString)
			if err != nil {
				return fmt.Errorf("failed to parse %s as NodeID: %w", nodeIDString, err)
			}
			stateSyncIDs[i] = nodeID
		}
	}

	// Initialize the state sync client
	v.Client = vmsync.NewClient(&vmsync.ClientConfig{
		StateSyncDone: v.stateSyncDone,
		Chain:         v.eth,
		State:         v.State,
		Client: statesyncclient.NewClient(
			&statesyncclient.ClientConfig{
				NetworkClient:    v.Network,
				Codec:            v.networkCodec,
				Stats:            stats.NewClientSyncerStats(leafMetricsNames),
				StateSyncNodeIDs: stateSyncIDs,
				BlockParser:      v,
			},
		),
		Enabled:            stateSyncEnabled,
		SkipResume:         v.config.StateSyncSkipResume,
		MinBlocks:          v.config.StateSyncMinBlocks,
		RequestSize:        v.config.StateSyncRequestSize,
		LastAcceptedHeight: lastAcceptedHeight, // TODO clean up how this is passed around
		ChainDB:            v.chaindb,
		VerDB:              v.versiondb,
		MetadataDB:         v.metadataDB,
		Acceptor:           v,
		Parser:             v.extensionConfig.SyncableParser,
		Extender:           v.extensionConfig.SyncExtender,
	})

	// If StateSync is disabled, clear any ongoing summary so that we will not attempt to resume
	// sync using a snapshot that has been modified by the node running normal operations.
	if !stateSyncEnabled {
		return v.Client.ClearOngoingSummary()
	}

	return nil
}

func (v *VM) initChainState(lastAcceptedBlock *types.Block) error {
	block, err := wrapBlock(lastAcceptedBlock, v)
	if err != nil {
		return fmt.Errorf("failed to create block wrapper for the last accepted block: %w", err)
	}

	config := &chain.Config{
		DecidedCacheSize:      decidedCacheSize,
		MissingCacheSize:      missingCacheSize,
		UnverifiedCacheSize:   unverifiedCacheSize,
		BytesToIDCacheSize:    bytesToIDCacheSize,
		GetBlock:              v.getBlock,
		UnmarshalBlock:        v.parseBlock,
		BuildBlock:            v.buildBlock,
		BuildBlockWithContext: v.buildBlockWithContext,
		LastAcceptedBlock:     block,
	}

	// Register chain state metrics
	chainStateRegisterer := prometheus.NewRegistry()
	state, err := chain.NewMeteredState(chainStateRegisterer, config)
	if err != nil {
		return fmt.Errorf("could not create metered state: %w", err)
	}
	v.State = state

	if !metrics.Enabled() {
		return nil
	}

	return v.metricsGatherer.Register(chainStateMetricsPrefix, chainStateRegisterer)
}

func (v *VM) SetState(_ context.Context, state uint32) error {
	switch vm.State(state) {
	case vm.Syncing:
		v.bootstrapped.Set(false)
		return nil
	case vm.Bootstrapping:
		return v.onBootstrapStarted()
	case vm.Ready:
		return v.onNormalOperationsStarted()
	default:
		return consensus.ErrUnknownState
	}
}

// onBootstrapStarted marks this VM as bootstrapping
func (v *VM) onBootstrapStarted() error {
	v.bootstrapped.Set(false)
	if err := v.Client.Error(); err != nil {
		return err
	}
	// After starting bootstrapping, do not attempt to resume a previous state sync.
	if err := v.Client.ClearOngoingSummary(); err != nil {
		return err
	}
	// Ensure snapshots are initialized before bootstrapping (i.e., if state sync is skipped).
	// Note calling this function has no effect if snapshots are already initialized.
	v.blockChain.InitializeSnapshots()
	return nil
}

// onNormalOperationsStarted marks this VM as bootstrapped
func (v *VM) onNormalOperationsStarted() error {
	if v.bootstrapped.Get() {
		return nil
	}
	v.bootstrapped.Set(true)
	// Initialize goroutines related to block building
	// once we enter normal operation as there is no need to handle mempool gossip before this point.
	return v.initBlockBuilding()
}

// initBlockBuilding starts goroutines to manage block building
func (v *VM) initBlockBuilding() error {
	ctx, cancel := context.WithCancel(context.TODO())
	v.cancel = cancel

	ethTxGossipMarshaller := GossipEthTxMarshaller{}
	ethTxGossipClient := v.Network.NewClient(p2p.TxGossipHandlerID, p2p.WithValidatorSampling(v.P2PValidators()))
	ethTxGossipMetrics, err := p2pgossip.NewMetrics(v.sdkMetrics, ethTxGossipNamespace)
	if err != nil {
		return fmt.Errorf("failed to initialize eth tx gossip metrics: %w", err)
	}
	ethTxPool, err := NewGossipEthTxPool(v.txPool, v.sdkMetrics)
	if err != nil {
		return fmt.Errorf("failed to initialize gossip eth tx pool: %w", err)
	}
	v.shutdownWg.Add(1)
	go func() {
		ethTxPool.Subscribe(ctx)
		v.shutdownWg.Done()
	}()
	pushGossipParams := p2pgossip.BranchingFactor{
		StakePercentage: v.config.PushGossipPercentStake,
		Validators:      v.config.PushGossipNumValidators,
		Peers:           v.config.PushGossipNumPeers,
	}
	pushRegossipParams := p2pgossip.BranchingFactor{
		Validators: v.config.PushRegossipNumValidators,
		Peers:      v.config.PushRegossipNumPeers,
	}

	ethTxPushGossiper, err := p2pgossip.NewPushGossiper[*GossipEthTx](
		ethTxGossipMarshaller,
		ethTxPool,
		v.P2PValidators(),
		ethTxGossipClient,
		ethTxGossipMetrics,
		pushGossipParams,
		pushRegossipParams,
		config.PushGossipDiscardedElements,
		config.TxGossipTargetMessageSize,
		v.config.RegossipFrequency.Duration,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize eth tx push gossiper: %w", err)
	}
	v.ethTxPushGossiper.Set(ethTxPushGossiper)

	// NOTE: gossip network must be initialized first otherwise ETH tx gossip will not work.
	v.builderLock.Lock()
	v.builder = v.NewBlockBuilder(v.extensionConfig.ExtraMempool)
	v.builder.awaitSubmittedTxs()

	// Start automining if enabled (dev mode)
	if v.config.EnableAutomining {
		v.builder.startAutomining(AutominingConfig{
			BuildBlock: func(ctx context.Context) (interface {
				Verify(context.Context) error
				Accept(context.Context) error
			}, error) {
				return v.buildBlock(ctx)
			},
			Interval: 100 * time.Millisecond,
		})
	}
	v.builderLock.Unlock()

	v.ethTxGossipHandler = gossip.NewTxGossipHandler[*GossipEthTx](
		v.ctxLogger,
		ethTxGossipMarshaller,
		ethTxPool,
		ethTxGossipMetrics,
		config.TxGossipTargetMessageSize,
		config.TxGossipThrottlingPeriod,
		config.TxGossipThrottlingLimit,
		v.P2PValidators(),
		nil, // BloomChecker - not needed for tx gossip
	)

	if err := v.Network.AddHandler(p2p.TxGossipHandlerID, v.ethTxGossipHandler); err != nil {
		return fmt.Errorf("failed to add eth tx gossip handler: %w", err)
	}

	ethTxPullGossiper := p2pgossip.NewPullGossiper[*GossipEthTx](
		v.ctxLogger,
		ethTxGossipMarshaller,
		ethTxPool,
		ethTxGossipClient,
		ethTxGossipMetrics,
		config.TxGossipPollSize,
	)

	v.ethTxPullGossiper = p2pgossip.ValidatorGossiper{
		Gossiper:   ethTxPullGossiper,
		NodeID:     v.ctx.NodeID,
		Validators: v.P2PValidators(),
	}

	v.shutdownWg.Add(1)
	go func() {
		p2pgossip.Every(ctx, v.ctxLogger, ethTxPushGossiper, v.config.PushGossipFrequency.Duration)
		v.shutdownWg.Done()
	}()
	v.shutdownWg.Add(1)
	go func() {
		p2pgossip.Every(ctx, v.ctxLogger, v.ethTxPullGossiper, v.config.PullGossipFrequency.Duration)
		v.shutdownWg.Done()
	}()

	v.shutdownWg.Add(1)
	go func() {
		p2pgossip.Every(ctx, v.ctxLogger, ethTxPushGossiper, v.config.PushGossipFrequency.Duration)
		v.shutdownWg.Done()
	}()
	v.shutdownWg.Add(1)
	go func() {
		p2pgossip.Every(ctx, v.ctxLogger, v.ethTxPullGossiper, v.config.PullGossipFrequency.Duration)
		v.shutdownWg.Done()
	}()

	return nil
}

func (v *VM) WaitForEvent(ctx context.Context) (interface{}, error) {
	v.builderLock.Lock()
	builder := v.builder
	v.builderLock.Unlock()

	// Block building is not initialized yet, so we haven't finished syncing or bootstrapping.
	if builder == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-v.stateSyncDone:
			return vm.Message{Type: vm.StateSyncDone}, nil
		case <-v.shutdownChan:
			return nil, errShuttingDownVM
		}
	}

	return builder.waitForEvent(ctx)
}

// Shutdown implements the quasarman.ChainVM interface
func (v *VM) Shutdown(context.Context) error {
	if v.ctx == nil {
		return nil
	}
	if v.cancel != nil {
		v.cancel()
	}
	v.Network.Shutdown()
	if err := v.Client.Shutdown(); err != nil {
		log.Error("error stopping state syncer", "err", err)
	}
	close(v.shutdownChan)
	// Stop RPC handlers before eth.Stop which will close the database
	for _, handler := range v.rpcHandlers {
		handler.Stop()
	}
	v.eth.Stop()
	v.shutdownWg.Wait()
	return nil
}

// Connected handles a peer connecting to the VM. This shadows the embedded
// Network.Connected method to satisfy the block.ChainVM interface which
// expects interface{} for the version parameter.
func (v *VM) Connected(ctx context.Context, nodeID ids.NodeID, nodeVersion interface{}) error {
	ver, ok := nodeVersion.(*consensusversion.Application)
	if !ok && nodeVersion != nil {
		return fmt.Errorf("expected *consensusversion.Application, got %T", nodeVersion)
	}
	return v.Network.Connected(ctx, nodeID, ver)
}

// Disconnected handles a peer disconnecting from the VM.
func (v *VM) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return v.Network.Disconnected(ctx, nodeID)
}

// buildBlock builds a block to be wrapped by ChainState
func (v *VM) buildBlock(ctx context.Context) (block.Block, error) {
	return v.buildBlockWithContext(ctx, nil)
}

func (v *VM) buildBlockWithContext(ctx context.Context, proposerVMBlockCtx *block.Context) (block.Block, error) {
	if proposerVMBlockCtx != nil {
		log.Debug("Building block with context", "pChainBlockHeight", proposerVMBlockCtx.PChainHeight)
	} else {
		log.Debug("Building block without context")
	}
	predicateCtx := &precompileconfig.PredicateContext{
		ConsensusCtx:       v.ctx,
		ProposerVMBlockCtx: proposerVMBlockCtx,
	}

	block, err := v.miner.GenerateBlock(predicateCtx)
	v.builder.handleGenerateBlock()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", vmerrors.ErrGenerateBlockFailed, err)
	}

	// Note: the status of block is set by ChainState
	blk, err := wrapBlock(block, v)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", vmerrors.ErrWrapBlockFailed, err)
	}
	// Verify is called on a non-wrapped block here, such that this
	// does not add [blk] to the processing blocks map in ChainState.
	//
	// TODO cache verification since Verify() will be called by the
	// consensus engine as well.
	//
	// Note: this is only called when building a new block, so caching
	// verification will only be a significant optimization for nodes
	// that produce a large number of blocks.
	// We call verify without writes here to avoid generating a reference
	// to the blk state root in the triedb when we are going to call verify
	// again from the consensus engine with writes enabled.
	if err := blk.verify(predicateCtx, false /*=writes*/); err != nil {
		return nil, fmt.Errorf("%w: %w", vmerrors.ErrBlockVerificationFailed, err)
	}

	log.Debug("built block",
		"id", blk.ID(),
	)
	return blk, nil
}

// parseBlock parses [b] into a block to be wrapped by ChainState.
func (v *VM) parseBlock(_ context.Context, b []byte) (block.Block, error) {
	ethBlock := new(types.Block)
	if err := rlp.DecodeBytes(b, ethBlock); err != nil {
		return nil, err
	}

	// Note: the status of block is set by ChainState
	block, err := wrapBlock(ethBlock, v)
	if err != nil {
		return nil, err
	}
	// Performing syntactic verification in ParseBlock allows for
	// short-circuiting bad blocks before they are processed by the VM.
	if err := block.syntacticVerify(); err != nil {
		return nil, fmt.Errorf("syntactic block verification failed: %w", err)
	}
	return block, nil
}

func (v *VM) ParseEthBlock(b []byte) (*types.Block, error) {
	block, err := v.parseBlock(context.TODO(), b)
	if err != nil {
		return nil, err
	}

	return block.(*wrappedBlock).ethBlock, nil
}

// getBlock attempts to retrieve block [id] from the VM to be wrapped
// by ChainState.
func (v *VM) getBlock(_ context.Context, id ids.ID) (block.Block, error) {
	ethBlock := v.blockChain.GetBlockByHash(common.Hash(id))
	// If [ethBlock] is nil, return [database.ErrNotFound] here
	// so that the miss is considered cacheable.
	if ethBlock == nil {
		return nil, database.ErrNotFound
	}
	// Note: the status of block is set by ChainState
	return wrapBlock(ethBlock, v)
}

// GetAcceptedBlock attempts to retrieve block [blkID] from the VM. This method
// only returns accepted blocks.
func (v *VM) GetAcceptedBlock(ctx context.Context, blkID ids.ID) (block.Block, error) {
	blk, err := v.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}

	height := blk.Height()
	acceptedBlkID, err := v.GetBlockIDAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	if acceptedBlkID != blkID {
		// The provided block is not accepted.
		return nil, database.ErrNotFound
	}
	return blk, nil
}

// SetPreference sets what the current tail of the chain is
func (v *VM) SetPreference(ctx context.Context, blkID ids.ID) error {
	// Since each internal handler used by [v.State] always returns a block
	// with non-nil ethBlock value, GetExtendedBlock should never return a
	// (*Block) with a nil ethBlock value.
	block, err := v.GetExtendedBlock(ctx, blkID)
	if err != nil {
		return fmt.Errorf("failed to set preference to %s: %w", blkID, err)
	}

	return v.blockChain.SetPreference(block.GetEthBlock())
}

// GetBlockIDAtHeight returns the canonical block at [height].
// Note: the engine assumes that if a block is not found at [height], then
// [database.ErrNotFound] will be returned. This indicates that the VM has state
// synced and does not have all historical blocks available.
func (v *VM) GetBlockIDAtHeight(_ context.Context, height uint64) (ids.ID, error) {
	lastAcceptedBlock := v.LastAcceptedBlock()
	if lastAcceptedBlock.Height() < height {
		return ids.ID{}, database.ErrNotFound
	}

	hash := v.blockChain.GetCanonicalHash(height)
	if hash == (common.Hash{}) {
		return ids.ID{}, database.ErrNotFound
	}
	return ids.ID(hash), nil
}

func (v *VM) Version(context.Context) (string, error) {
	return Version, nil
}

// CreateHandlers makes new http handlers that can handle API calls
func (v *VM) CreateHandlers(context.Context) (map[string]http.Handler, error) {
	handler := rpc.NewServer(v.config.APIMaxDuration.Duration)
	if v.config.HttpBodyLimit > 0 {
		handler.SetHTTPBodyLimit(int(v.config.HttpBodyLimit))
	}

	enabledAPIs := v.config.EthAPIs()
	if err := attachEthService(handler, v.eth.APIs(), enabledAPIs); err != nil {
		return nil, err
	}

	apis := make(map[string]http.Handler)

	if v.config.AdminAPIEnabled {
		adminAPI, err := utilsrpc.NewHandler("admin", NewAdminService(v, os.ExpandEnv(fmt.Sprintf("%s_coreth_performance_%s", v.config.AdminAPIDir, v.chainAlias))))
		if err != nil {
			return nil, fmt.Errorf("failed to register service for admin API due to %w", err)
		}
		apis[adminEndpoint] = adminAPI
		enabledAPIs = append(enabledAPIs, "coreth-admin")

		// Also register eth.AdminAPI with geth RPC server for underscore notation (admin_importChain)
		// This enables block import via RPC with proper persistence callback
		if err := handler.RegisterName("admin", eth.NewAdminAPI(v.eth)); err != nil {
			return nil, fmt.Errorf("failed to register eth admin API: %w", err)
		}
		log.Info("Registered eth admin API with geth RPC server (admin_importChain enabled)")
	}

	if v.config.WarpAPIEnabled {
		warpSDKClient := v.Network.NewClient(p2p.SignatureRequestHandlerID)
		signatureAggregator := luxwarp.NewSignatureAggregator(v.ctxLogger, warpSDKClient)

		if err := handler.RegisterName("warp", warp.NewAPI(v.ctx, v.warpBackend, signatureAggregator, v.requirePrimaryNetworkSigners)); err != nil {
			return nil, err
		}
		enabledAPIs = append(enabledAPIs, "warp")
	}

	log.Info("enabling apis",
		"apis", enabledAPIs,
	)
	apis[ethRPCEndpoint] = handler
	apis[ethWSEndpoint] = handler.WebsocketHandlerWithDuration(
		[]string{"*"},
		v.config.APIMaxDuration.Duration,
		v.config.WSCPURefillRate.Duration,
		v.config.WSCPUMaxStored.Duration,
	)

	v.rpcHandlers = append(v.rpcHandlers, handler)
	return apis, nil
}

func (*VM) NewHTTPHandler(context.Context) (interface{}, error) {
	return nil, nil
}

func (v *VM) chainConfigExtra() *extras.ChainConfig {
	return params.GetExtra(v.chainConfig)
}

func (v *VM) rules(number *big.Int, time uint64) extras.Rules {
	ethrules := v.chainConfig.Rules(number, params.IsMergeTODO, time)
	return *params.GetRulesExtra(ethrules)
}

// currentRules returns the chain rules for the current block.
func (v *VM) currentRules() extras.Rules {
	header := v.eth.APIBackend.CurrentHeader()
	return v.rules(header.Number, header.Time)
}

// requirePrimaryNetworkSigners returns true if warp messages from the primary
// network must be signed by the primary network validators.
// This is necessary when the subnet is not validating the primary network.
func (v *VM) requirePrimaryNetworkSigners() bool {
	switch c := v.currentRules().Precompiles[warpcontract.ContractAddress].(type) {
	case *warpcontract.Config:
		return c.RequirePrimaryNetworkSigners
	default: // includes nil due to non-presence
		return false
	}
}

func (v *VM) startContinuousProfiler() {
	// If the profiler directory is empty, return immediately
	// without creating or starting a continuous profiler.
	if v.config.ContinuousProfilerDir == "" {
		return
	}
	v.profiler = profiler.NewContinuous(
		filepath.Join(v.config.ContinuousProfilerDir),
		v.config.ContinuousProfilerFrequency.Duration,
		v.config.ContinuousProfilerMaxFiles,
	)
	defer v.profiler.Shutdown()

	v.shutdownWg.Add(1)
	go func() {
		defer v.shutdownWg.Done()
		log.Info("Dispatching continuous profiler", "dir", v.config.ContinuousProfilerDir, "freq", v.config.ContinuousProfilerFrequency, "maxFiles", v.config.ContinuousProfilerMaxFiles)
		err := v.profiler.Dispatch()
		if err != nil {
			log.Error("continuous profiler failed", "err", err)
		}
	}()
	// Wait for shutdownChan to be closed
	<-v.shutdownChan
}

// readLastAccepted reads the last accepted hash from [acceptedBlockDB] and returns the
// last accepted block hash and height by reading directly from [v.chaindb] instead of relying
// on [chain].
// Note: assumes [v.chaindb] and [v.genesisHash] have been initialized.
func (v *VM) ReadLastAccepted() (common.Hash, uint64, error) {
	// Attempt to load last accepted block to determine if it is necessary to
	// initialize state with the genesis block.
	lastAcceptedBytes, lastAcceptedErr := v.acceptedBlockDB.Get(lastAcceptedKey)
	switch {
	case lastAcceptedErr == database.ErrNotFound:
		// If there is nothing in the database, return the genesis block hash and height
		return v.genesisHash, 0, nil
	case lastAcceptedErr != nil:
		return common.Hash{}, 0, fmt.Errorf("failed to get last accepted block ID due to: %w", lastAcceptedErr)
	case len(lastAcceptedBytes) != common.HashLength:
		return common.Hash{}, 0, fmt.Errorf("last accepted bytes should have been length %d, but found %d", common.HashLength, len(lastAcceptedBytes))
	default:
		lastAcceptedHash := common.BytesToHash(lastAcceptedBytes)
		height, found := rawdb.ReadHeaderNumber(v.chaindb, lastAcceptedHash)
		if !found {
			return common.Hash{}, 0, fmt.Errorf("failed to retrieve header number of last accepted block: %s", lastAcceptedHash)
		}
		return lastAcceptedHash, height, nil
	}
}

// attachEthService registers the backend RPC services provided by Ethereum
// to the provided handler under their assigned namespaces.
func attachEthService(handler *rpc.Server, apis []rpc.API, names []string) error {
	enabledServicesSet := make(map[string]struct{})
	for _, ns := range names {
		// handle pre geth v1.10.20 api names as aliases for their updated values
		// to allow configurations to be backwards compatible.
		if newName, isLegacy := legacyApiNames[ns]; isLegacy {
			log.Info("deprecated api name referenced in configuration.", "deprecated", ns, "new", newName)
			enabledServicesSet[newName] = struct{}{}
			continue
		}

		enabledServicesSet[ns] = struct{}{}
	}

	apiSet := make(map[string]rpc.API)
	for _, api := range apis {
		if existingAPI, exists := apiSet[api.Name]; exists {
			return fmt.Errorf("duplicated API name: %s, namespaces %s and %s", api.Name, api.Namespace, existingAPI.Namespace)
		}
		apiSet[api.Name] = api
	}

	for name := range enabledServicesSet {
		api, exists := apiSet[name]
		if !exists {
			return fmt.Errorf("API service %s not found", name)
		}
		if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
			return err
		}
	}

	return nil
}

func (v *VM) stateSyncEnabled(lastAcceptedHeight uint64) bool {
	if v.config.StateSyncEnabled != nil {
		// if the config is set, use that
		return *v.config.StateSyncEnabled
	}

	// enable state sync by default if the chain is empty.
	return lastAcceptedHeight == 0
}

// importChainData imports blockchain data from a specified path
// Supports both database directories (raw copy) and RLP files (validated import)
func (v *VM) importChainData(dataPath string) error {
	// Check if this is an RLP file
	if strings.HasSuffix(strings.ToLower(dataPath), ".rlp") {
		// RLP files need blockchain initialized first - mark for later import
		log.Info("RLP import detected, will import after blockchain initialization", "path", dataPath)
		return nil // RLP import happens in importRLPBlocks after chain init
	}

	// Open the source database
	sourceDB, err := openSourceDatabase(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Find the last block in the source database
	var lastBlockNumber uint64
	var lastBlockHash common.Hash

	// Iterate through the source database to find the highest block
	it := sourceDB.NewIterator([]byte("h"), nil)
	defer it.Release()

	for it.Next() {
		// Block header keys are prefixed with 'h' followed by block number (8 bytes) and hash (32 bytes)
		key := it.Key()
		if len(key) == 41 && key[0] == 'h' {
			blockNumber := binary.BigEndian.Uint64(key[1:9])
			if blockNumber > lastBlockNumber {
				lastBlockNumber = blockNumber
				copy(lastBlockHash[:], key[9:41])
			}
		}
	}

	if lastBlockNumber == 0 {
		log.Warn("No blocks found in import database")
		return nil
	}

	log.Info("Found last block in import database", "number", lastBlockNumber, "hash", lastBlockHash)

	// Get the block header to verify
	headerData := rawdb.ReadHeaderRLP(sourceDB, lastBlockHash, lastBlockNumber)
	if len(headerData) == 0 {
		return fmt.Errorf("could not read header for block %d", lastBlockNumber)
	}

	// Copy all data from source to destination
	log.Info("Copying blockchain data...")

	batch := v.chaindb.NewBatch()
	count := 0

	// Use a new iterator to copy all data
	copyIt := sourceDB.NewIterator(nil, nil)
	defer copyIt.Release()

	for copyIt.Next() {
		if err := batch.Put(copyIt.Key(), copyIt.Value()); err != nil {
			return fmt.Errorf("failed to write key: %w", err)
		}

		count++
		if count%10000 == 0 {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write batch: %w", err)
			}
			batch.Reset()
			log.Info("Import progress", "keys", count)
		}
	}

	// Write final batch
	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write final batch: %w", err)
	}

	// Set the last accepted block
	if err := v.acceptedBlockDB.Put(lastAcceptedKey, lastBlockHash[:]); err != nil {
		return fmt.Errorf("failed to set last accepted: %w", err)
	}

	log.Info("Chain data import complete",
		"totalKeys", count,
		"lastBlock", lastBlockNumber,
		"lastHash", lastBlockHash,
	)

	return nil
}

// importRLPBlocks imports blocks from an RLP file with full validation
func (v *VM) importRLPBlocks(rlpPath string) error {
	log.Info("Starting RLP block import", "path", rlpPath)

	file, err := os.Open(rlpPath)
	if err != nil {
		return fmt.Errorf("failed to open RLP file: %w", err)
	}
	defer file.Close()

	// Get file size for progress reporting
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// Create RLP stream reader
	stream := rlp.NewStream(file, 0)

	var (
		blocks   []*types.Block
		imported uint64
		skipped  uint64
		batch    = 100 // Import blocks in batches
		start    = time.Now()
	)

	currentHeight := v.blockChain.CurrentBlock().Number.Uint64()
	log.Info("Current chain height", "height", currentHeight)

	for {
		var block types.Block
		err := stream.Decode(&block)
		if err == io.EOF {
			break
		}
		if err != nil {
			// Try to continue past decoding errors
			log.Warn("Error decoding block, skipping", "error", err)
			skipped++
			continue
		}

		// Skip blocks we already have
		if block.NumberU64() <= currentHeight {
			skipped++
			continue
		}

		blocks = append(blocks, &block)

		// Insert blocks in batches
		if len(blocks) >= batch {
			n, err := v.blockChain.InsertChain(blocks)
			if err != nil {
				log.Error("Failed to insert blocks",
					"first", blocks[0].NumberU64(),
					"last", blocks[len(blocks)-1].NumberU64(),
					"inserted", n,
					"error", err,
				)
				// Continue with remaining blocks if partial insert
				if n > 0 {
					imported += uint64(n)
					blocks = blocks[n:]
				}
				// Skip problematic block and continue
				if len(blocks) > 0 {
					log.Warn("Skipping problematic block", "number", blocks[0].NumberU64())
					blocks = blocks[1:]
					skipped++
				}
				continue
			}
			imported += uint64(len(blocks))

			// Progress reporting
			pos, _ := file.Seek(0, io.SeekCurrent)
			progress := float64(pos) / float64(fileSize) * 100
			elapsed := time.Since(start)
			rate := float64(imported) / elapsed.Seconds()

			log.Info("Import progress",
				"imported", imported,
				"skipped", skipped,
				"progress", fmt.Sprintf("%.1f%%", progress),
				"rate", fmt.Sprintf("%.1f blocks/sec", rate),
				"lastBlock", blocks[len(blocks)-1].NumberU64(),
			)

			blocks = blocks[:0]
		}
	}

	// Insert remaining blocks
	if len(blocks) > 0 {
		n, err := v.blockChain.InsertChain(blocks)
		if err != nil {
			log.Error("Failed to insert final blocks", "error", err, "inserted", n)
		}
		imported += uint64(n)
	}

	elapsed := time.Since(start)
	log.Info("RLP import complete",
		"imported", imported,
		"skipped", skipped,
		"elapsed", elapsed,
		"rate", fmt.Sprintf("%.1f blocks/sec", float64(imported)/elapsed.Seconds()),
	)

	// Update lastAcceptedKey to mark imported blocks as accepted
	// This is critical - without this, imported blocks exist in DB but aren't
	// recognized by the consensus layer
	if imported > 0 {
		currentBlock := v.blockChain.CurrentBlock()
		if currentBlock != nil {
			// CRITICAL: Accept and commit the state trie for the imported blocks
			// Without this, the state exists in memory but isn't persisted to disk,
			// causing "required historical state unavailable" errors on restart
			fullBlock := v.blockChain.GetBlock(currentBlock.Hash(), currentBlock.Number.Uint64())
			if fullBlock != nil {
				log.Info("Committing state for imported blocks", "height", fullBlock.NumberU64())
				if err := v.blockChain.AcceptImportedState(fullBlock); err != nil {
					return fmt.Errorf("failed to accept imported state: %w", err)
				}
			}

			currentHash := currentBlock.Hash()
			if err := v.acceptedBlockDB.Put(lastAcceptedKey, currentHash[:]); err != nil {
				return fmt.Errorf("failed to update last accepted block after import: %w", err)
			}
			// Commit the versiondb to persist the lastAccepted update
			if err := v.versiondb.Commit(); err != nil {
				return fmt.Errorf("failed to commit versiondb after import: %w", err)
			}
			log.Info("Updated last accepted block after RLP import",
				"hash", currentHash,
				"height", currentBlock.Number.Uint64(),
			)
		}
	}

	return nil
}

func (v *VM) PutLastAcceptedID(ID ids.ID) error {
	return v.acceptedBlockDB.Put(lastAcceptedKey, ID[:])
}
