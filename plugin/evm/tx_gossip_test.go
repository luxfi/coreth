// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"encoding/binary"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/coreth/plugin/evm/atomic"
	atomicvm "github.com/luxfi/coreth/plugin/evm/atomic/vm"

	luxatomic "github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/database/memdb"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/network/p2p"
	"github.com/luxfi/node/network/p2p/gossip"
	"github.com/luxfi/node/proto/pb/sdk"
	"github.com/luxfi/node/quasar"
	"github.com/luxfi/node/quasar/engine/enginetest"
	"github.com/luxfi/node/quasar/quasartest"
	"github.com/luxfi/node/quasar/validators"
	agoUtils "github.com/luxfi/node/utils"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/node/utils/logging"
	"github.com/luxfi/node/utils/set"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/luxfi/node/vms/components/lux"
	"github.com/luxfi/node/vms/secp256k1fx"

	"google.golang.org/protobuf/proto"

	"github.com/luxfi/coreth/plugin/evm/config"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap0"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/core/types"
)

func TestEthTxGossip(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)
	validatorState := utils.NewTestValidatorState()
	quasarCtx.ValidatorState = validatorState

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	responseSender := &enginetest.SenderStub{
		SentAppResponse: make(chan []byte, 1),
	}
	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		responseSender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	// sender for the peer requesting gossip from [vm]
	peerSender := &enginetest.SenderStub{
		SentAppRequest: make(chan []byte, 1),
	}

	network, err := p2p.NewNetwork(logging.NoLog{}, peerSender, prometheus.NewRegistry(), "")
	require.NoError(err)
	client := network.NewClient(p2p.TxGossipHandlerID)

	// we only accept gossip requests from validators
	requestingNodeID := ids.GenerateTestNodeID()
	require.NoError(vm.Connected(ctx, requestingNodeID, nil))
	validatorState.GetCurrentHeightF = func(context.Context) (uint64, error) {
		return 0, nil
	}
	validatorState.GetValidatorSetF = func(context.Context, uint64, ids.ID) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
		return map[ids.NodeID]*validators.GetValidatorOutput{
			requestingNodeID: {
				NodeID: requestingNodeID,
				Weight: 1,
			},
		}, nil
	}

	// Ask the VM for any new transactions. We should get nothing at first.
	emptyBloomFilter, err := gossip.NewBloomFilter(
		prometheus.NewRegistry(),
		"",
		config.TxGossipBloomMinTargetElements,
		config.TxGossipBloomTargetFalsePositiveRate,
		config.TxGossipBloomResetFalsePositiveRate,
	)
	require.NoError(err)
	emptyBloomFilterBytes, _ := emptyBloomFilter.Marshal()
	request := &sdk.PullGossipRequest{
		Filter: emptyBloomFilterBytes,
		Salt:   agoUtils.RandomBytes(32),
	}

	requestBytes, err := proto.Marshal(request)
	require.NoError(err)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	onResponse := func(_ context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		require.NoError(err)

		response := &sdk.PullGossipResponse{}
		require.NoError(proto.Unmarshal(responseBytes, response))
		require.Empty(response.Gossip)
		wg.Done()
	}
	require.NoError(client.AppRequest(ctx, set.Of(vm.Ctx.NodeID), requestBytes, onResponse))
	require.NoError(vm.AppRequest(ctx, requestingNodeID, 1, time.Time{}, <-peerSender.SentAppRequest))
	require.NoError(network.AppResponse(ctx, quasarCtx.NodeID, 1, <-responseSender.SentAppResponse))
	wg.Wait()

	// Issue a tx to the VM
	tx := types.NewTransaction(0, address, big.NewInt(10), 100_000, big.NewInt(ap0.MinGasPrice), nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(innerVM.chainID), pk.ToECDSA())
	require.NoError(err)

	errs := innerVM.txPool.Add([]*types.Transaction{signedTx}, true, true)
	require.Len(errs, 1)
	require.Nil(errs[0])

	// wait so we aren't throttled by the vm
	time.Sleep(5 * time.Second)

	marshaller := GossipEthTxMarshaller{}
	// Ask the VM for new transactions. We should get the newly issued tx.
	wg.Add(1)
	onResponse = func(_ context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		require.NoError(err)

		response := &sdk.PullGossipResponse{}
		require.NoError(proto.Unmarshal(responseBytes, response))
		require.Len(response.Gossip, 1)

		gotTx, err := marshaller.UnmarshalGossip(response.Gossip[0])
		require.NoError(err)
		require.Equal(signedTx.Hash(), gotTx.Tx.Hash())

		wg.Done()
	}
	require.NoError(client.AppRequest(ctx, set.Of(vm.Ctx.NodeID), requestBytes, onResponse))
	require.NoError(vm.AppRequest(ctx, requestingNodeID, 3, time.Time{}, <-peerSender.SentAppRequest))
	require.NoError(network.AppResponse(ctx, quasarCtx.NodeID, 3, <-responseSender.SentAppResponse))
	wg.Wait()
}

func TestAtomicTxGossip(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)
	quasarCtx.LUXAssetID = ids.GenerateTestID()
	validatorState := utils.NewTestValidatorState()
	quasarCtx.ValidatorState = validatorState
	memory := luxatomic.NewMemory(memdb.New())
	quasarCtx.SharedMemory = memory.NewSharedMemory(quasarCtx.ChainID)

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	responseSender := &enginetest.SenderStub{
		SentAppResponse: make(chan []byte, 1),
	}
	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		responseSender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	// sender for the peer requesting gossip from [vm]
	peerSender := &enginetest.SenderStub{
		SentAppRequest: make(chan []byte, 1),
	}
	network, err := p2p.NewNetwork(logging.NoLog{}, peerSender, prometheus.NewRegistry(), "")
	require.NoError(err)
	client := network.NewClient(p2p.AtomicTxGossipHandlerID)

	// we only accept gossip requests from validators
	requestingNodeID := ids.GenerateTestNodeID()
	require.NoError(vm.Connected(ctx, requestingNodeID, nil))
	validatorState.GetCurrentHeightF = func(context.Context) (uint64, error) {
		return 0, nil
	}
	validatorState.GetValidatorSetF = func(context.Context, uint64, ids.ID) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
		return map[ids.NodeID]*validators.GetValidatorOutput{
			requestingNodeID: {
				NodeID: requestingNodeID,
				Weight: 1,
			},
		}, nil
	}

	// Ask the VM for any new transactions. We should get nothing at first.
	emptyBloomFilter, err := gossip.NewBloomFilter(
		prometheus.NewRegistry(),
		"",
		config.TxGossipBloomMinTargetElements,
		config.TxGossipBloomTargetFalsePositiveRate,
		config.TxGossipBloomResetFalsePositiveRate,
	)
	require.NoError(err)
	emptyBloomFilterBytes, _ := emptyBloomFilter.Marshal()
	request := &sdk.PullGossipRequest{
		Filter: emptyBloomFilterBytes,
		Salt:   agoUtils.RandomBytes(32),
	}

	requestBytes, err := proto.Marshal(request)
	require.NoError(err)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	onResponse := func(_ context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		require.NoError(err)

		response := &sdk.PullGossipResponse{}
		require.NoError(proto.Unmarshal(responseBytes, response))
		require.Empty(response.Gossip)
		wg.Done()
	}
	require.NoError(client.AppRequest(ctx, set.Of(vm.Ctx.NodeID), requestBytes, onResponse))
	require.NoError(vm.AppRequest(ctx, requestingNodeID, 1, time.Time{}, <-peerSender.SentAppRequest))
	require.NoError(network.AppResponse(ctx, quasarCtx.NodeID, 1, <-responseSender.SentAppResponse))
	wg.Wait()

	// Issue a tx to the VM
	utxo, err := addUTXO(
		memory,
		quasarCtx,
		ids.GenerateTestID(),
		0,
		quasarCtx.LUXAssetID,
		100_000_000_000,
		pk.Address(),
	)
	require.NoError(err)
	tx, err := atomic.NewImportTx(vm.Ctx, vm.CurrentRules(), vm.Clock().Unix(), vm.Ctx.XChainID, address, initialBaseFee, secp256k1fx.NewKeychain(pk), []*lux.UTXO{utxo})
	require.NoError(err)
	require.NoError(vm.AtomicMempool.AddLocalTx(tx))

	// wait so we aren't throttled by the vm
	time.Sleep(5 * time.Second)

	// Ask the VM for new transactions. We should get the newly issued tx.
	wg.Add(1)

	marshaller := atomic.TxMarshaller{}
	onResponse = func(_ context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		require.NoError(err)

		response := &sdk.PullGossipResponse{}
		require.NoError(proto.Unmarshal(responseBytes, response))
		require.Len(response.Gossip, 1)

		gotTx, err := marshaller.UnmarshalGossip(response.Gossip[0])
		require.NoError(err)
		require.Equal(tx.ID(), gotTx.GossipID())

		wg.Done()
	}
	require.NoError(client.AppRequest(ctx, set.Of(vm.Ctx.NodeID), requestBytes, onResponse))
	require.NoError(vm.AppRequest(ctx, requestingNodeID, 3, time.Time{}, <-peerSender.SentAppRequest))
	require.NoError(network.AppResponse(ctx, quasarCtx.NodeID, 3, <-responseSender.SentAppResponse))
	wg.Wait()
}

// Tests that a tx is gossiped when it is issued
func TestEthTxPushGossipOutbound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)
	sender := &enginetest.SenderStub{
		SentAppGossip: make(chan []byte, 1),
	}

	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		sender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	tx := types.NewTransaction(0, address, big.NewInt(10), 100_000, big.NewInt(ap0.MinGasPrice), nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(innerVM.chainID), pk.ToECDSA())
	require.NoError(err)

	// issue a tx
	require.NoError(innerVM.txPool.Add([]*types.Transaction{signedTx}, true, true)[0])
	innerVM.ethTxPushGossiper.Get().Add(&GossipEthTx{signedTx})

	sent := <-sender.SentAppGossip
	got := &sdk.PushGossip{}

	// we should get a message that has the protocol prefix and the gossip
	// message
	require.Equal(byte(p2p.TxGossipHandlerID), sent[0])
	require.NoError(proto.Unmarshal(sent[1:], got))

	marshaller := GossipEthTxMarshaller{}
	require.Len(got.Gossip, 1)
	gossipedTx, err := marshaller.UnmarshalGossip(got.Gossip[0])
	require.NoError(err)
	require.Equal(ids.ID(signedTx.Hash()), gossipedTx.GossipID())
}

// Tests that a gossiped tx is added to the mempool and forwarded
func TestEthTxPushGossipInbound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)

	sender := &enginetest.Sender{}
	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		sender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	tx := types.NewTransaction(0, address, big.NewInt(10), 100_000, big.NewInt(ap0.MinGasPrice), nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(innerVM.chainID), pk.ToECDSA())
	require.NoError(err)

	marshaller := GossipEthTxMarshaller{}
	gossipedTx := &GossipEthTx{
		Tx: signedTx,
	}
	gossipedTxBytes, err := marshaller.MarshalGossip(gossipedTx)
	require.NoError(err)

	inboundGossip := &sdk.PushGossip{
		Gossip: [][]byte{gossipedTxBytes},
	}

	inboundGossipBytes, err := proto.Marshal(inboundGossip)
	require.NoError(err)

	inboundGossipMsg := append(binary.AppendUvarint(nil, p2p.TxGossipHandlerID), inboundGossipBytes...)
	require.NoError(vm.AppGossip(ctx, ids.EmptyNodeID, inboundGossipMsg))

	require.True(innerVM.txPool.Has(signedTx.Hash()))
}

// Tests that a tx is gossiped when it is issued
func TestAtomicTxPushGossipOutbound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)
	quasarCtx.LUXAssetID = ids.GenerateTestID()
	validatorState := utils.NewTestValidatorState()
	quasarCtx.ValidatorState = validatorState
	memory := luxatomic.NewMemory(memdb.New())
	quasarCtx.SharedMemory = memory.NewSharedMemory(quasarCtx.ChainID)

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	sender := &enginetest.SenderStub{
		SentAppGossip: make(chan []byte, 1),
	}
	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		sender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	// Issue a tx to the VM
	utxo, err := addUTXO(
		memory,
		quasarCtx,
		ids.GenerateTestID(),
		0,
		quasarCtx.LUXAssetID,
		100_000_000_000,
		pk.Address(),
	)
	require.NoError(err)
	tx, err := atomic.NewImportTx(vm.Ctx, vm.CurrentRules(), vm.Clock().Unix(), vm.Ctx.XChainID, address, initialBaseFee, secp256k1fx.NewKeychain(pk), []*lux.UTXO{utxo})
	require.NoError(err)
	require.NoError(vm.AtomicMempool.AddLocalTx(tx))
	vm.AtomicTxPushGossiper.Add(tx)

	gossipedBytes := <-sender.SentAppGossip
	require.Equal(byte(p2p.AtomicTxGossipHandlerID), gossipedBytes[0])

	outboundGossipMsg := &sdk.PushGossip{}
	require.NoError(proto.Unmarshal(gossipedBytes[1:], outboundGossipMsg))
	require.Len(outboundGossipMsg.Gossip, 1)

	marshaller := atomic.TxMarshaller{}
	gossipedTx, err := marshaller.UnmarshalGossip(outboundGossipMsg.Gossip[0])
	require.NoError(err)
	require.Equal(tx.ID(), gossipedTx.GossipID())
}

// Tests that a tx is gossiped when it is issued
func TestAtomicTxPushGossipInbound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	quasarCtx := quasartest.Context(t, quasartest.CChainID)
	quasarCtx.LUXAssetID = ids.GenerateTestID()
	validatorState := utils.NewTestValidatorState()
	quasarCtx.ValidatorState = validatorState
	memory := luxatomic.NewMemory(memdb.New())
	quasarCtx.SharedMemory = memory.NewSharedMemory(quasarCtx.ChainID)

	pk, err := secp256k1.NewPrivateKey()
	require.NoError(err)
	address := pk.EthAddress()
	genesis := newPrefundedGenesis(100_000_000_000_000_000, address)
	genesisBytes, err := genesis.MarshalJSON()
	require.NoError(err)

	sender := &enginetest.Sender{}
	innerVM := &VM{}
	vm := atomicvm.WrapVM(innerVM)

	require.NoError(vm.Initialize(
		ctx,
		quasarCtx,
		memdb.New(),
		genesisBytes,
		nil,
		nil,
		nil,
		sender,
	))
	require.NoError(vm.SetState(ctx, quasar.NormalOp))

	defer func() {
		require.NoError(vm.Shutdown(ctx))
	}()

	// issue a tx to the vm
	utxo, err := addUTXO(
		memory,
		quasarCtx,
		ids.GenerateTestID(),
		0,
		quasarCtx.LUXAssetID,
		100_000_000_000,
		pk.Address(),
	)
	require.NoError(err)
	tx, err := atomic.NewImportTx(vm.Ctx, vm.CurrentRules(), vm.Clock().Unix(), vm.Ctx.XChainID, address, initialBaseFee, secp256k1fx.NewKeychain(pk), []*lux.UTXO{utxo})
	require.NoError(err)
	require.NoError(vm.AtomicMempool.AddLocalTx(tx))

	marshaller := atomic.TxMarshaller{}
	gossipBytes, err := marshaller.MarshalGossip(tx)
	require.NoError(err)

	inboundGossip := &sdk.PushGossip{
		Gossip: [][]byte{gossipBytes},
	}
	inboundGossipBytes, err := proto.Marshal(inboundGossip)
	require.NoError(err)

	inboundGossipMsg := append(binary.AppendUvarint(nil, p2p.AtomicTxGossipHandlerID), inboundGossipBytes...)

	require.NoError(vm.AppGossip(ctx, ids.EmptyNodeID, inboundGossipMsg))
	require.True(vm.AtomicMempool.Has(tx.ID()))
}
