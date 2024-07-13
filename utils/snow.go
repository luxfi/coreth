// (c) 2019-2020, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/luxfi/node/api/metrics"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/snow"
	"github.com/luxfi/node/snow/validators"
	"github.com/luxfi/node/utils/crypto/bls"
	"github.com/luxfi/node/utils/logging"
)

func TestSnowContext() *snow.Context {
	sk, err := bls.NewSecretKey()
	if err != nil {
		panic(err)
	}
	pk := bls.PublicFromSecretKey(sk)
	return &snow.Context{
		NetworkID:      0,
		SubnetID:       ids.Empty,
		ChainID:        ids.Empty,
		NodeID:         ids.EmptyNodeID,
		PublicKey:      pk,
		Log:            logging.NoLog{},
		BCLookup:       ids.NewAliaser(),
		Metrics:        metrics.NewMultiGatherer(),
		ChainDataDir:   "",
		ValidatorState: &validators.TestState{},
	}
}
