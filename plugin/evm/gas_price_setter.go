// (c) 2021-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"math/big"

	"github.com/luxfi/geth/core/txpool"
)

// gasPriceSetterWrapper wraps a TxPool to implement gasPriceSetter
type gasPriceSetterWrapper struct {
	pool *txpool.TxPool
}

func (w *gasPriceSetterWrapper) SetGasPrice(price *big.Int) {
	// The TxPool in newer versions might not have SetGasPrice
	// This is a no-op for compatibility
}

func (w *gasPriceSetterWrapper) SetMinFee(price *big.Int) {
	// The TxPool in newer versions might not have SetMinFee
	// This is a no-op for compatibility
}