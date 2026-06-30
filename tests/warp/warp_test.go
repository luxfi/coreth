// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build warp_e2e

// Implements solidity tests.
package warp

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/stretchr/testify/require"

	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"

	validators "github.com/luxfi/consensus/validator"
	"github.com/luxfi/constants"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/vms/platformvm"
	"github.com/luxfi/node/vms/platformvm/api"
	"github.com/luxfi/vm/api/info"
	"github.com/luxfi/vm/tests/fixture/e2e"
	"github.com/luxfi/vm/tests/fixture/tmpnet"
	"github.com/luxfi/warp"
	"github.com/luxfi/warp/payload"

	"github.com/luxfi/coreth/accounts/abi/bind"
	"github.com/luxfi/coreth/cmd/simulator/key"
	"github.com/luxfi/coreth/cmd/simulator/load"
	"github.com/luxfi/coreth/cmd/simulator/metrics"
	"github.com/luxfi/coreth/cmd/simulator/txs"
	"github.com/luxfi/coreth/ethclient"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/coreth/precompile/contracts/warp"
	"github.com/luxfi/coreth/predicate"
	"github.com/luxfi/coreth/tests/utils"
	"github.com/luxfi/coreth/tests/warp/aggregator"
	warpBackend "github.com/luxfi/coreth/warp"
	ethereum "github.com/luxfi/geth"
	"github.com/luxfi/geth/core/types"
)

var (
	flagVars *e2e.FlagVars

	cChainChainDetails *Chain

	testPayload = []byte{1, 2, 3}
)

func init() {
	// Configures flags used to configure tmpnet (via SynchronizedBeforeSuite)
	flagVars = e2e.RegisterFlags()
}

// Chain provides the basic details of a created chain
type Chain struct {
	// ChainID is the txID of the transaction that created the chain
	ChainID ids.ID
	// For simplicity assume a single blockchain per chain
	BlockchainID ids.ID
	// Key funded in the genesis of the blockchain
	PreFundedKey *ecdsa.PrivateKey
	// ValidatorURIs are the base URIs for each participant of the Chain
	ValidatorURIs []string
}

func TestE2E(t *testing.T) {
	ginkgo.RunSpecs(t, "coreth warp e2e test")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	// Run only once in the first ginkgo process

	tc := e2e.NewTestContext()
	nodes := tmpnet.NewNodesOrPanic(tmpnet.DefaultNodeCount)

	env := e2e.NewTestEnvironment(
		tc,
		flagVars,
		utils.NewTmpnetNetwork(
			"coreth-warp-e2e",
			nodes,
			tmpnet.FlagsMap{},
		),
	)

	return env.Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()

	// Initialize the local test environment from the global state
	if len(envBytes) > 0 {
		e2e.InitSharedTestEnvironment(tc, envBytes)
	}

	network := e2e.GetEnv(tc).GetNetwork()

	// By default all nodes are validating all chains
	validatorURIs := make([]string, len(network.Nodes))
	for i, node := range network.Nodes {
		validatorURIs[i] = node.URI
	}

	infoClient := info.NewClient(network.Nodes[0].URI)
	cChainBlockchainID, err := infoClient.GetBlockchainID(tc.DefaultContext(), "C")
	require.NoError(err)

	cChainChainDetails = &Chain{
		ChainID:       constants.PrimaryNetworkID,
		BlockchainID:  cChainBlockchainID,
		PreFundedKey:  tmpnet.HardhatKey.ToECDSA(),
		ValidatorURIs: validatorURIs,
	}
})

var _ = ginkgo.Describe("[Warp]", func() {
	testFunc := func(sendingChain *Chain, receivingChain *Chain) {
		tc := e2e.NewTestContext()
		w := newWarpTest(tc.DefaultContext(), sendingChain, receivingChain)

		ginkgo.GinkgoLogr.Info("Sending message from A to B")
		w.sendMessageFromSendingChain()

		ginkgo.GinkgoLogr.Info("Aggregating signatures via API")
		w.aggregateSignaturesViaAPI()

		ginkgo.GinkgoLogr.Info("Aggregating signatures via p2p aggregator")
		w.aggregateSignatures()

		ginkgo.GinkgoLogr.Info("Delivering addressed call payload to receiving chain")
		w.deliverAddressedCallToReceivingChain()

		ginkgo.GinkgoLogr.Info("Delivering block hash payload to receiving chain")
		w.deliverBlockHashPayload()

		ginkgo.GinkgoLogr.Info("Executing warp load test")
		w.warpLoad()
	}
	// TODO: Uncomment these tests when we have a way to run them in CI, currently we should not depend on Chain-EVM
	// as Coreth and Chain-EVM have different release cycles. The problem is that once we update Luxd (protocol version),
	// we need to update Chain-EVM to the same protocol version. Until then all Chain-EVM tests are broken, so it's blocking Coreth development.
	// It's best to not run these tests until we have a way to run them in CI.
	// ginkgo.It("ChainA -> C-Chain", func() { testFunc(chainA, cChainChainDetails) })
	// ginkgo.It("C-Chain -> ChainA", func() { testFunc(cChainChainDetails, chainA) })
	ginkgo.It("C-Chain -> C-Chain", func() { testFunc(cChainChainDetails, cChainChainDetails) })
})

type warpTest struct {
	// network-wide fields set in the constructor
	networkID uint32

	// sendingChain fields set in the constructor
	sendingChain              *Chain
	sendingChainURIs          []string
	sendingChainClients       []*ethclient.Client
	sendingChainFundedKey     *ecdsa.PrivateKey
	sendingChainFundedAddress common.Address
	sendingChainChainID       *big.Int
	sendingChainSigner        types.Signer

	// receivingChain fields set in the constructor
	receivingChain              *Chain
	receivingChainURIs          []string
	receivingChainClients       []*ethclient.Client
	receivingChainFundedKey     *ecdsa.PrivateKey
	receivingChainFundedAddress common.Address
	receivingChainChainID       *big.Int
	receivingChainSigner        types.Signer

	// Fields set throughout test execution
	blockID                     ids.ID
	blockPayload                *payload.Hash
	blockPayloadUnsignedMessage *warp.UnsignedMessage
	blockPayloadSignedMessage   *warp.Message

	addressedCallUnsignedMessage *warp.UnsignedMessage
	addressedCallSignedMessage   *warp.Message
}

func newWarpTest(ctx context.Context, sendingChain *Chain, receivingChain *Chain) *warpTest {
	require := require.New(ginkgo.GinkgoT())

	sendingChainFundedKey := sendingChain.PreFundedKey
	receivingChainFundedKey := receivingChain.PreFundedKey

	warpTest := &warpTest{
		sendingChain:                sendingChain,
		sendingChainURIs:            sendingChain.ValidatorURIs,
		receivingChain:              receivingChain,
		receivingChainURIs:          receivingChain.ValidatorURIs,
		sendingChainFundedKey:       sendingChainFundedKey,
		sendingChainFundedAddress:   crypto.PubkeyToAddress(sendingChainFundedKey.PublicKey),
		receivingChainFundedKey:     receivingChainFundedKey,
		receivingChainFundedAddress: crypto.PubkeyToAddress(receivingChainFundedKey.PublicKey),
	}
	infoClient := info.NewClient(sendingChain.ValidatorURIs[0])
	networkID, err := infoClient.GetNetworkID(ctx)
	require.NoError(err)
	warpTest.networkID = networkID

	warpTest.initClients()

	sendingClient := warpTest.sendingChainClients[0]
	sendingChainChainID, err := sendingClient.ChainID(ctx)
	require.NoError(err)
	warpTest.sendingChainChainID = sendingChainChainID
	warpTest.sendingChainSigner = types.LatestSignerForChainID(sendingChainChainID)

	receivingClient := warpTest.receivingChainClients[0]
	receivingChainID, err := receivingClient.ChainID(ctx)
	require.NoError(err)
	// Issue transactions to activate ProposerVM on the receiving chain
	require.NoError(utils.IssueTxsToActivateProposerVMFork(ctx, receivingChainID, receivingChainFundedKey, receivingClient))
	warpTest.receivingChainChainID = receivingChainID
	warpTest.receivingChainSigner = types.LatestSignerForChainID(receivingChainID)

	return warpTest
}

func (w *warpTest) initClients() {
	require := require.New(ginkgo.GinkgoT())

	w.sendingChainClients = make([]*ethclient.Client, 0, len(w.sendingChainClients))
	for _, uri := range w.sendingChain.ValidatorURIs {
		wsURI := toWebsocketURI(uri, w.sendingChain.BlockchainID.String())
		ginkgo.GinkgoLogr.Info("Creating ethclient for blockchain A", "blockchainID", w.sendingChain.BlockchainID)
		client, err := ethclient.Dial(wsURI)
		require.NoError(err)
		w.sendingChainClients = append(w.sendingChainClients, client)
	}

	w.receivingChainClients = make([]*ethclient.Client, 0, len(w.receivingChainClients))
	for _, uri := range w.receivingChain.ValidatorURIs {
		wsURI := toWebsocketURI(uri, w.receivingChain.BlockchainID.String())
		ginkgo.GinkgoLogr.Info("Creating ethclient for blockchain B", "blockchainID", w.receivingChain.BlockchainID)
		client, err := ethclient.Dial(wsURI)
		require.NoError(err)
		w.receivingChainClients = append(w.receivingChainClients, client)
	}
}

func (w *warpTest) sendMessageFromSendingChain() {
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()
	require := require.New(ginkgo.GinkgoT())

	client := w.sendingChainClients[0]

	startingNonce, err := client.NonceAt(ctx, w.sendingChainFundedAddress, nil)
	require.NoError(err)

	packedInput, err := warp.PackSendWarpMessage(testPayload)
	require.NoError(err)
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   w.sendingChainChainID,
		Nonce:     startingNonce,
		To:        &warp.Module.Address,
		Gas:       200_000,
		GasFeeCap: big.NewInt(225 * params.GWei),
		GasTipCap: big.NewInt(params.GWei),
		Value:     common.Big0,
		Data:      packedInput,
	})
	signedTx, err := types.SignTx(tx, w.sendingChainSigner, w.sendingChainFundedKey)
	require.NoError(err)
	ginkgo.GinkgoLogr.Info("Sending sendWarpMessage transaction", "txHash", signedTx.Hash())
	require.NoError(client.SendTransaction(ctx, signedTx))

	ginkgo.GinkgoLogr.Info("Waiting for transaction to be accepted")
	receiptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(receiptCtx, client, signedTx)
	require.NoError(err)
	blockHash := receipt.BlockHash
	blockNumber := receipt.BlockNumber.Uint64()

	ginkgo.GinkgoLogr.Info("Constructing warp block hash unsigned message", "blockHash", blockHash)
	w.blockID = ids.ID(blockHash) // Set blockID to construct a warp message containing a block hash payload later
	w.blockPayload, err = payload.NewHash(w.blockID[:])
	require.NoError(err)
	w.blockPayloadUnsignedMessage, err = warp.NewUnsignedMessage(w.networkID, w.sendingChain.BlockchainID, w.blockPayload.Bytes())
	require.NoError(err)

	ginkgo.GinkgoLogr.Info("Fetching relevant warp logs from the newly produced block")
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
		Addresses: []common.Address{warp.Module.Address},
	})
	require.NoError(err)
	require.Len(logs, 1)

	// Check for relevant warp log from subscription and ensure that it matches
	// the log extracted from the last block.
	txLog := logs[0]
	ginkgo.GinkgoLogr.Info("Parsing logData as unsigned warp message")
	unsignedMsg, err := warp.UnpackSendWarpEventDataToMessage(txLog.Data)
	require.NoError(err)

	// Set local variables for the duration of the test
	w.addressedCallUnsignedMessage = unsignedMsg
	ginkgo.GinkgoLogr.Info("Parsed unsignedWarpMsg", "unsignedWarpMessageID", w.addressedCallUnsignedMessage.ID(), "unsignedWarpMessage", w.addressedCallUnsignedMessage)

	// Loop over each client on chain A to ensure they all have time to accept the block.
	// Note: if we did not confirm this here, the next stage could be racy since it assumes every node
	// has accepted the block.
	for i, client := range w.sendingChainClients {
		// Loop until each node has advanced to >= the height of the block that emitted the warp log
		for {
			receivedBlkNum, err := client.BlockNumber(ctx)
			require.NoError(err)
			if receivedBlkNum >= blockNumber {
				ginkgo.GinkgoLogr.Info("client accepted the block containing SendWarpMessage", "client", i, "height", receivedBlkNum)
				break
			}
		}
	}
}

func (w *warpTest) aggregateSignaturesViaAPI() {
	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()

	warpAPIs := make(map[ids.NodeID]warpBackend.Client, len(w.sendingChainURIs))
	for _, uri := range w.sendingChainURIs {
		client, err := warpBackend.NewClient(uri, w.sendingChain.BlockchainID.String())
		require.NoError(err)

		infoClient := info.NewClient(uri)
		nodeID, _, err := infoClient.GetNodeID(ctx)
		require.NoError(err)
		warpAPIs[nodeID] = client
	}

	pChainClient := platformvm.NewClient(w.sendingChainURIs[0])
	pChainHeight, err := pChainClient.GetHeight(ctx)
	require.NoError(err)
	// If the source chain is the Primary Network, then we only need to aggregate signatures from the receiving
	// chain's validator set instead of the entire Primary Network.
	// If the destination turns out to be the Primary Network as well, then this is a no-op.
	var validators map[ids.NodeID]*validators.GetValidatorOutput
	if w.sendingChain.ChainID == constants.PrimaryNetworkID {
		validators, err = pChainClient.GetValidatorsAt(ctx, w.receivingChain.ChainID, api.Height(pChainHeight))
	} else {
		validators, err = pChainClient.GetValidatorsAt(ctx, w.sendingChain.ChainID, api.Height(pChainHeight))
	}
	require.NoError(err)
	require.NotZero(len(validators))

	totalWeight := uint64(0)
	warpValidators := make([]*warp.Validator, 0, len(validators))
	for nodeID, validator := range validators {
		warpValidators = append(warpValidators, &warp.Validator{
			PublicKey: validator.PublicKey,
			Weight:    validator.Weight,
			NodeID:    nodeID,
		})
		totalWeight += validator.Weight
	}

	ginkgo.GinkgoLogr.Info("Aggregating signatures from validator set", "numValidators", len(warpValidators), "totalWeight", totalWeight)
	apiSignatureGetter := NewAPIFetcher(warpAPIs)
	signatureResult, err := aggregator.New(apiSignatureGetter, warpValidators, totalWeight).AggregateSignatures(ctx, w.addressedCallUnsignedMessage, 100)
	require.NoError(err)
	require.Equal(signatureResult.SignatureWeight, signatureResult.TotalWeight)
	require.Equal(signatureResult.SignatureWeight, totalWeight)

	w.addressedCallSignedMessage = signatureResult.Message

	signatureResult, err = aggregator.New(apiSignatureGetter, warpValidators, totalWeight).AggregateSignatures(ctx, w.blockPayloadUnsignedMessage, 100)
	require.NoError(err)
	require.Equal(signatureResult.SignatureWeight, signatureResult.TotalWeight)
	require.Equal(signatureResult.SignatureWeight, totalWeight)
	w.blockPayloadSignedMessage = signatureResult.Message

	ginkgo.GinkgoLogr.Info("Aggregated signatures for warp messages", "addressedCallMessage", common.Bytes2Hex(w.addressedCallSignedMessage.Bytes()), "blockPayloadMessage", common.Bytes2Hex(w.blockPayloadSignedMessage.Bytes()))
}

func (w *warpTest) aggregateSignatures() {
	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()

	// Verify that the signature aggregation matches the results of manually constructing the warp message
	client, err := warpBackend.NewClient(w.sendingChainURIs[0], w.sendingChain.BlockchainID.String())
	require.NoError(err)

	ginkgo.GinkgoLogr.Info("Fetching addressed call aggregate signature via p2p API")
	chainIDStr := ""
	if w.sendingChain.ChainID == constants.PrimaryNetworkID {
		chainIDStr = w.receivingChain.ChainID.String()
	}
	signedWarpMessageBytes, err := client.GetMessageAggregateSignature(ctx, w.addressedCallSignedMessage.ID(), warp.WarpQuorumDenominator, chainIDStr)
	require.NoError(err)
	require.Equal(w.addressedCallSignedMessage.Bytes(), signedWarpMessageBytes)

	ginkgo.GinkgoLogr.Info("Fetching block payload aggregate signature via p2p API")
	signedWarpBlockBytes, err := client.GetBlockAggregateSignature(ctx, w.blockID, warp.WarpQuorumDenominator, chainIDStr)
	require.NoError(err)
	require.Equal(w.blockPayloadSignedMessage.Bytes(), signedWarpBlockBytes)
}

func (w *warpTest) deliverAddressedCallToReceivingChain() {
	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()

	client := w.receivingChainClients[0]

	nonce, err := client.NonceAt(ctx, w.receivingChainFundedAddress, nil)
	require.NoError(err)

	packedInput, err := warp.PackGetVerifiedWarpMessage(0)
	require.NoError(err)
	tx := predicate.NewPredicateTx(
		w.receivingChainChainID,
		nonce,
		&warp.Module.Address,
		5_000_000,
		big.NewInt(225*params.GWei),
		big.NewInt(params.GWei),
		common.Big0,
		packedInput,
		types.AccessList{},
		warp.ContractAddress,
		w.addressedCallSignedMessage.Bytes(),
	)
	signedTx, err := types.SignTx(tx, w.receivingChainSigner, w.receivingChainFundedKey)
	require.NoError(err)
	txBytes, err := signedTx.MarshalBinary()
	require.NoError(err)
	ginkgo.GinkgoLogr.Info("Sending getVerifiedWarpMessage transaction", "txHash", signedTx.Hash(), "txBytes", common.Bytes2Hex(txBytes))
	require.NoError(client.SendTransaction(ctx, signedTx))

	ginkgo.GinkgoLogr.Info("Waiting for transaction to be accepted")
	receiptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(receiptCtx, client, signedTx)
	require.NoError(err)
	blockHash := receipt.BlockHash

	ginkgo.GinkgoLogr.Info("Fetching relevant warp logs and receipts from new block")
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
		Addresses: []common.Address{warp.Module.Address},
	})
	require.NoError(err)
	require.Len(logs, 0)
	require.NoError(err)
	require.Equal(receipt.Status, types.ReceiptStatusSuccessful)
}

func (w *warpTest) deliverBlockHashPayload() {
	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()

	client := w.receivingChainClients[0]

	nonce, err := client.NonceAt(ctx, w.receivingChainFundedAddress, nil)
	require.NoError(err)

	packedInput, err := warp.PackGetVerifiedWarpBlockHash(0)
	require.NoError(err)
	tx := predicate.NewPredicateTx(
		w.receivingChainChainID,
		nonce,
		&warp.Module.Address,
		5_000_000,
		big.NewInt(225*params.GWei),
		big.NewInt(params.GWei),
		common.Big0,
		packedInput,
		types.AccessList{},
		warp.ContractAddress,
		w.blockPayloadSignedMessage.Bytes(),
	)
	signedTx, err := types.SignTx(tx, w.receivingChainSigner, w.receivingChainFundedKey)
	require.NoError(err)
	txBytes, err := signedTx.MarshalBinary()
	require.NoError(err)
	ginkgo.GinkgoLogr.Info("Sending getVerifiedWarpBlockHash transaction", "txHash", signedTx.Hash(), "txBytes", common.Bytes2Hex(txBytes))
	require.NoError(client.SendTransaction(ctx, signedTx))

	ginkgo.GinkgoLogr.Info("Waiting for transaction to be accepted")
	receiptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(receiptCtx, client, signedTx)
	require.NoError(err)
	blockHash := receipt.BlockHash
	ginkgo.GinkgoLogr.Info("Fetching relevant warp logs and receipts from new block")
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
		Addresses: []common.Address{warp.Module.Address},
	})
	require.NoError(err)
	require.Len(logs, 0)
	require.NoError(err)
	require.Equal(receipt.Status, types.ReceiptStatusSuccessful)
}

func (w *warpTest) warpLoad() {
	require := require.New(ginkgo.GinkgoT())
	tc := e2e.NewTestContext()
	ctx := tc.DefaultContext()

	var (
		numWorkers           = len(w.sendingChainClients)
		txsPerWorker  uint64 = 10
		batchSize     uint64 = 10
		sendingClient        = w.sendingChainClients[0]
	)

	chainAKeys, chainAPrivateKeys := generateKeys(w.sendingChainFundedKey, numWorkers)
	chainBKeys, chainBPrivateKeys := generateKeys(w.receivingChainFundedKey, numWorkers)

	loadMetrics := metrics.NewDefaultMetrics()

	ginkgo.GinkgoLogr.Info("Distributing funds on sending chain", "numKeys", len(chainAKeys))
	chainAKeys, err := load.DistributeFunds(ctx, sendingClient, chainAKeys, len(chainAKeys), new(big.Int).Mul(big.NewInt(100), big.NewInt(params.Ether)), loadMetrics)
	require.NoError(err)

	ginkgo.GinkgoLogr.Info("Distributing funds on receiving chain", "numKeys", len(chainBKeys))
	_, err = load.DistributeFunds(ctx, w.receivingChainClients[0], chainBKeys, len(chainBKeys), new(big.Int).Mul(big.NewInt(100), big.NewInt(params.Ether)), loadMetrics)
	require.NoError(err)

	ginkgo.GinkgoLogr.Info("Creating workers for each chain...")
	chainAWorkers := make([]txs.Worker[*types.Transaction], 0, len(chainAKeys))
	for i := range chainAKeys {
		chainAWorkers = append(chainAWorkers, load.NewTxReceiptWorker(ctx, w.sendingChainClients[i]))
	}
	chainBWorkers := make([]txs.Worker[*types.Transaction], 0, len(chainBKeys))
	for i := range chainBKeys {
		chainBWorkers = append(chainBWorkers, load.NewTxReceiptWorker(ctx, w.receivingChainClients[i]))
	}

	ginkgo.GinkgoLogr.Info("Subscribing to warp send events on sending chain")
	logs := make(chan types.Log, numWorkers*int(txsPerWorker))
	sub, err := sendingClient.SubscribeFilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{warp.Module.Address},
	}, logs)
	require.NoError(err)
	defer func() {
		sub.Unsubscribe()
		require.NoError(<-sub.Err())
	}()

	ginkgo.GinkgoLogr.Info("Generating tx sequence to send warp messages...")
	warpSendSequences, err := txs.GenerateTxSequences(ctx, func(key *ecdsa.PrivateKey, nonce uint64) (*types.Transaction, error) {
		data, err := warp.PackSendWarpMessage([]byte(fmt.Sprintf("Jets %d-%d Dolphins", key.X.Int64(), nonce)))
		if err != nil {
			return nil, err
		}
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   w.sendingChainChainID,
			Nonce:     nonce,
			To:        &warp.Module.Address,
			Gas:       200_000,
			GasFeeCap: big.NewInt(225 * params.GWei),
			GasTipCap: big.NewInt(params.GWei),
			Value:     common.Big0,
			Data:      data,
		})
		return types.SignTx(tx, w.sendingChainSigner, key)
	}, w.sendingChainClients[0], chainAPrivateKeys, txsPerWorker, false)
	require.NoError(err)
	ginkgo.GinkgoLogr.Info("Executing warp send loader...")
	warpSendLoader := load.New(chainAWorkers, warpSendSequences, batchSize, loadMetrics)
	// TODO: execute send and receive loaders concurrently.
	require.NoError(warpSendLoader.Execute(ctx))
	require.NoError(warpSendLoader.ConfirmReachedTip(ctx))

	warpClient, err := warpBackend.NewClient(w.sendingChainURIs[0], w.sendingChain.BlockchainID.String())
	require.NoError(err)
	chainIDStr := ""
	if w.sendingChain.ChainID == constants.PrimaryNetworkID {
		chainIDStr = w.receivingChain.ChainID.String()
	}

	ginkgo.GinkgoLogr.Info("Executing warp delivery sequences...")
	warpDeliverSequences, err := txs.GenerateTxSequences(ctx, func(key *ecdsa.PrivateKey, nonce uint64) (*types.Transaction, error) {
		// Wait for the next warp send log
		warpLog := <-logs

		unsignedMessage, err := warp.UnpackSendWarpEventDataToMessage(warpLog.Data)
		if err != nil {
			return nil, err
		}
		ginkgo.GinkgoLogr.Info("Fetching addressed call aggregate signature via p2p API")

		signedWarpMessageBytes, err := warpClient.GetMessageAggregateSignature(ctx, unsignedMessage.ID(), warp.WarpDefaultQuorumNumerator, chainIDStr)
		if err != nil {
			return nil, err
		}

		packedInput, err := warp.PackGetVerifiedWarpMessage(0)
		if err != nil {
			return nil, err
		}
		tx := predicate.NewPredicateTx(
			w.receivingChainChainID,
			nonce,
			&warp.Module.Address,
			5_000_000,
			big.NewInt(225*params.GWei),
			big.NewInt(params.GWei),
			common.Big0,
			packedInput,
			types.AccessList{},
			warp.ContractAddress,
			signedWarpMessageBytes,
		)
		return types.SignTx(tx, w.receivingChainSigner, key)
	}, w.receivingChainClients[0], chainBPrivateKeys, txsPerWorker, true)
	require.NoError(err)

	ginkgo.GinkgoLogr.Info("Executing warp delivery...")
	warpDeliverLoader := load.New(chainBWorkers, warpDeliverSequences, batchSize, loadMetrics)
	require.NoError(warpDeliverLoader.Execute(ctx))
	require.NoError(warpSendLoader.ConfirmReachedTip(ctx))
	ginkgo.GinkgoLogr.Info("Completed warp delivery successfully.")
}

func generateKeys(preFundedKey *ecdsa.PrivateKey, numWorkers int) ([]*key.Key, []*ecdsa.PrivateKey) {
	keys := []*key.Key{
		key.CreateKey(preFundedKey),
	}
	privateKeys := []*ecdsa.PrivateKey{
		preFundedKey,
	}
	for i := 1; i < numWorkers; i++ {
		newKey, err := key.Generate()
		require.NoError(ginkgo.GinkgoT(), err)
		keys = append(keys, newKey)
		privateKeys = append(privateKeys, newKey.PrivKey)
	}
	return keys, privateKeys
}

func toWebsocketURI(uri string, blockchainID string) string {
	return fmt.Sprintf("ws://%s/ext/bc/%s/ws", strings.TrimPrefix(uri, "http://"), blockchainID)
}
