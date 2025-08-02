// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/luxfi/coreth/accounts/abi/bind"
	"github.com/luxfi/coreth/ethclient"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap1"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/crypto"
	"github.com/luxfi/log"
	ethparams "github.com/luxfi/geth/params"
)

const expectedBlockHeight = 2

// IssueTxsToActivateProposerVMFork issues transactions at the current
// timestamp, which should be after the ProposerVM activation time (aka
// ApricotPhase4). This should generate a PostForkBlock because its parent block
// (genesis) has a timestamp (0) that is greater than or equal to the fork
// activation time of 0. Therefore, subsequent blocks should be built with
// BuildBlockWithContext.
func IssueTxsToActivateProposerVMFork(
	ctx context.Context, chainID *big.Int, fundedKey *ecdsa.PrivateKey,
	client *ethclient.Client,
) error {
	addr := crypto.PubkeyToAddress(fundedKey.PublicKey)
	nonce, err := client.NonceAt(ctx, addr, nil)
	if err != nil {
		return err
	}

	gasPrice := big.NewInt(ap1.MinGasPrice) // should be pretty generous for c-chain and subnets
	txSigner := types.LatestSignerForChainID(chainID)

	// Send exactly 2 transactions, waiting for each to be included in a block
	for i := 0; i < expectedBlockHeight; i++ {
		tx := types.NewTransaction(
			nonce, addr, common.Big1, ethparams.TxGas, gasPrice, nil)
		triggerTx, err := types.SignTx(tx, txSigner, fundedKey)
		if err != nil {
			return err
		}
		if err := client.SendTransaction(ctx, triggerTx); err != nil {
			return err
		}

		// Wait for this transaction to be included in a block
		receiptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if _, err := bind.WaitMined(receiptCtx, client, triggerTx); err != nil {
			return err
		}
		nonce++
	}

	log.Info(
		"Built sufficient blocks to activate proposerVM fork",
		"blockCount", expectedBlockHeight,
	)
	return nil
}
