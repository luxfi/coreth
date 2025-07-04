// (c) 2023, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"fmt"
	"testing"

	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/coreth/precompile/testutils"
	"github.com/luxfi/coreth/utils"
	"go.uber.org/mock/gomock"
)

func TestVerify(t *testing.T) {
	tests := map[string]testutils.ConfigVerifyTest{
		"quorum numerator less than minimum": {
			Config:        NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum-1),
			ExpectedError: fmt.Sprintf("cannot specify quorum numerator (%d) < min quorum numerator (%d)", WarpQuorumNumeratorMinimum-1, WarpQuorumNumeratorMinimum),
		},
		"quorum numerator greater than quorum denominator": {
			Config:        NewConfig(utils.NewUint64(3), WarpQuorumDenominator+1),
			ExpectedError: fmt.Sprintf("cannot specify quorum numerator (%d) > quorum denominator (%d)", WarpQuorumDenominator+1, WarpQuorumDenominator),
		},
		"default quorum numerator": {
			Config: NewDefaultConfig(utils.NewUint64(3)),
		},
		"valid quorum numerator 1 less than denominator": {
			Config: NewConfig(utils.NewUint64(3), WarpQuorumDenominator-1),
		},
		"valid quorum numerator 1 more than minimum": {
			Config: NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum+1),
		},
		"invalid cannot activated before Durango activation": {
			Config: NewConfig(utils.NewUint64(3), 0),
			ChainConfig: func() precompileconfig.ChainConfig {
				config := precompileconfig.NewMockChainConfig(gomock.NewController(t))
				config.EXPECT().IsDurango(gomock.Any()).Return(false)
				return config
			}(),
			ExpectedError: errWarpCannotBeActivated.Error(),
		},
	}
	testutils.RunVerifyTests(t, tests)
}

func TestEqualWarpConfig(t *testing.T) {
	tests := map[string]testutils.ConfigEqualTest{
		"non-nil config and nil other": {
			Config:   NewDefaultConfig(utils.NewUint64(3)),
			Other:    nil,
			Expected: false,
		},

		"different type": {
			Config:   NewDefaultConfig(utils.NewUint64(3)),
			Other:    precompileconfig.NewMockConfig(gomock.NewController(t)),
			Expected: false,
		},

		"different timestamp": {
			Config:   NewDefaultConfig(utils.NewUint64(3)),
			Other:    NewDefaultConfig(utils.NewUint64(4)),
			Expected: false,
		},

		"different quorum numerator": {
			Config:   NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum+1),
			Other:    NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum+2),
			Expected: false,
		},

		"same default config": {
			Config:   NewDefaultConfig(utils.NewUint64(3)),
			Other:    NewDefaultConfig(utils.NewUint64(3)),
			Expected: true,
		},

		"same non-default config": {
			Config:   NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum+5),
			Other:    NewConfig(utils.NewUint64(3), WarpQuorumNumeratorMinimum+5),
			Expected: true,
		},
	}
	testutils.RunEqualTests(t, tests)
}
