// (c) 2019-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/utils/set"

	commonEng "github.com/luxfi/node/snow/engine/common"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/stretchr/testify/assert"

	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/types"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/coreth/plugin/evm/message"
)

func fundAddressByGenesis(addrs []common.Address) (string, error) {
	balance := big.NewInt(0xffffffffffffff)
	genesis := &core.Genesis{
		Difficulty: common.Big0,
		GasLimit:   uint64(5000000),
	}
	funds := make(map[common.Address]core.GenesisAccount)
	for _, addr := range addrs {
		funds[addr] = core.GenesisAccount{
			Balance: balance,
		}
	}
	genesis.Alloc = funds
	genesis.Config = params.TestChainConfig

	bytes, err := json.Marshal(genesis)
	return string(bytes), err
}

func getValidEthTxs(key *ecdsa.PrivateKey, count int, gasPrice *big.Int) []*types.Transaction {
	res := make([]*types.Transaction, count)

	to := common.Address{}
	amount := big.NewInt(0)
	gasLimit := uint64(37000)

	for i := 0; i < count; i++ {
		tx, _ := types.SignTx(
			types.NewTransaction(
				uint64(i),
				to,
				amount,
				gasLimit,
				gasPrice,
				[]byte(strings.Repeat("aaaaaaaaaa", 100))),
			types.HomesteadSigner{}, key)
		tx.SetTime(time.Now().Add(-1 * time.Minute))
		res[i] = tx
	}
	return res
}

// show that a geth tx discovered from gossip is requested to the same node that
// gossiped it
func TestMempoolEthTxsAppGossipHandling(t *testing.T) {
	if os.Getenv("RUN_FLAKY_TESTS") != "true" {
		t.Skip("FLAKY")
	}
	assert := assert.New(t)

	key, err := crypto.GenerateKey()
	assert.NoError(err)

	addr := crypto.PubkeyToAddress(key.PublicKey)

	genesisJSON, err := fundAddressByGenesis([]common.Address{addr})
	assert.NoError(err)

	_, vm, _, _, sender := GenesisVM(t, true, genesisJSON, "", "")
	defer func() {
		err := vm.Shutdown(context.Background())
		assert.NoError(err)
	}()
	vm.txPool.SetGasTip(common.Big1)
	vm.txPool.SetMinFee(common.Big0)

	var (
		wg          sync.WaitGroup
		txRequested bool
	)
	sender.CantSendAppGossip = false
	sender.SendAppRequestF = func(context.Context, set.Set[ids.NodeID], uint32, []byte) error {
		txRequested = true
		return nil
	}
	wg.Add(1)
	sender.SendAppGossipF = func(context.Context, commonEng.SendConfig, []byte) error {
		wg.Done()
		return nil
	}

	// prepare a tx
	tx := getValidEthTxs(key, 1, common.Big1)[0]

	// show that unknown coreth hashes is requested
	txBytes, err := rlp.EncodeToBytes([]*types.Transaction{tx})
	assert.NoError(err)
	msg := message.EthTxsGossip{
		Txs: txBytes,
	}
	msgBytes, err := message.BuildGossipMessage(vm.networkCodec, msg)
	assert.NoError(err)

	nodeID := ids.GenerateTestNodeID()
	err = vm.AppGossip(context.Background(), nodeID, msgBytes)
	assert.NoError(err)
	assert.False(txRequested, "tx should not be requested")

	// wait for transaction to be re-gossiped
	attemptAwait(t, &wg, 5*time.Second)
}

func attemptAwait(t *testing.T, wg *sync.WaitGroup, delay time.Duration) {
	ticker := make(chan struct{})

	// Wait for [wg] and then close [ticket] to indicate that
	// the wait group has finished.
	go func() {
		wg.Wait()
		close(ticker)
	}()

	select {
	case <-time.After(delay):
		t.Fatal("Timed out waiting for wait group to complete")
	case <-ticker:
		// The wait group completed without issue
	}
}
