// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"

	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/utils/set"
)

type validatorSet struct {
	set set.Set[ids.NodeID]
}

func (v *validatorSet) Has(ctx context.Context, nodeID ids.NodeID) bool {
	return v.set.Contains(nodeID)
}
