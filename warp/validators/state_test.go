// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"testing"

	"github.com/luxfi/constants"
	"github.com/luxfi/ids"

	validators "github.com/luxfi/consensus/validator"
	"github.com/luxfi/consensus/validator/validatorsmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetValidatorSetPrimaryNetwork(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	myChainID := ids.GenerateTestID()
	otherChainID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	mockState := validatorsmock.NewState(ctrl)
	state := NewState(mockState, myChainID, chainID, false)

	// Expect that requesting my validator set returns my validator set
	mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), myChainID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	output, err := state.GetValidatorSet(context.Background(), 10, myChainID)
	require.NoError(err)
	require.Len(output, 0)

	// Expect that requesting the Primary Network validator set overrides and returns my validator set
	mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), myChainID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	output, err = state.GetValidatorSet(context.Background(), 10, constants.PrimaryNetworkID)
	require.NoError(err)
	require.Len(output, 0)

	// Expect that requesting other validator set returns that validator set
	mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), otherChainID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	output, err = state.GetValidatorSet(context.Background(), 10, otherChainID)
	require.NoError(err)
	require.Len(output, 0)
}
