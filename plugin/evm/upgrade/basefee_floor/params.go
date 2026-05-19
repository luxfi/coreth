// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package basefee_floor

import "github.com/luxfi/coreth/utils"

// MinBaseFee is the minimum base fee specified in ACP-125 that is allowed after
//
// See: https://github.com/lux-foundation/ACPs/tree/main/ACPs/125-basefee-reduction
//
// This value modifies the previously used `blockgascost.MinBaseFee`.
const MinBaseFee = utils.GWei
