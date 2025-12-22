// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"testing"
	"time"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/database/memdb"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/cache"
	"github.com/luxfi/node/cache/lru"
	"github.com/luxfi/p2p"
	"github.com/luxfi/p2p/lp118"
	"github.com/luxfi/warp"
	"github.com/luxfi/warp/payload"

	"github.com/luxfi/coreth/metrics/metricstest"
	"github.com/luxfi/coreth/warp/warptest"
	"github.com/stretchr/testify/require"
)

const testNetworkID uint32 = 369

// testWarpSigner wraps a warp.Signer to implement lp118.Signer
type testWarpSigner struct {
	signer warp.Signer
}

func (s *testWarpSigner) Sign(msg *warp.UnsignedMessage) ([]byte, error) {
	sig, err := s.signer.Sign(msg)
	if err != nil {
		return nil, err
	}
	return sig[:], nil
}

func TestAddressedCallSignatures(t *testing.T) {
	metricstest.WithMetrics(t)

	database := memdb.New()
	chainID := ids.GenerateTestID()

	// Create BLS key and signer
	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	warpSigner := warp.NewSigner(sk, testNetworkID, chainID)
	lp118Signer := &testWarpSigner{signer: warpSigner}

	offChainPayload, err := payload.NewAddressedCall([]byte{1, 2, 3}, []byte{1, 2, 3})
	require.NoError(t, err)
	offchainMessage, err := warp.NewUnsignedMessage(testNetworkID, chainID, offChainPayload.Bytes())
	require.NoError(t, err)
	offchainSignature, err := warpSigner.Sign(offchainMessage)
	require.NoError(t, err)

	tests := map[string]struct {
		setup       func(backend Backend) (request []byte, expectedResponse []byte)
		verifyStats func(t *testing.T, stats *verifierStats)
		errCode     int32 // 0 means no error expected
	}{
		"known message": {
			setup: func(backend Backend) (request []byte, expectedResponse []byte) {
				knownPayload, err := payload.NewAddressedCall([]byte{0, 0, 0}, []byte("test"))
				require.NoError(t, err)
				msg, err := warp.NewUnsignedMessage(testNetworkID, chainID, knownPayload.Bytes())
				require.NoError(t, err)
				signature, err := warpSigner.Sign(msg)
				require.NoError(t, err)

				backend.AddMessage(msg)
				return msg.Bytes(), signature[:]
			},
			verifyStats: func(t *testing.T, stats *verifierStats) {
				require.EqualValues(t, 0, stats.messageParseFail.Snapshot().Count())
				require.EqualValues(t, 0, stats.blockValidationFail.Snapshot().Count())
			},
		},
		"offchain message": {
			setup: func(_ Backend) (request []byte, expectedResponse []byte) {
				return offchainMessage.Bytes(), offchainSignature[:]
			},
			verifyStats: func(t *testing.T, stats *verifierStats) {
				require.EqualValues(t, 0, stats.messageParseFail.Snapshot().Count())
				require.EqualValues(t, 0, stats.blockValidationFail.Snapshot().Count())
			},
		},
		"unknown message": {
			setup: func(_ Backend) (request []byte, expectedResponse []byte) {
				unknownPayload, err := payload.NewAddressedCall([]byte{0, 0, 0}, []byte("unknown message"))
				require.NoError(t, err)
				unknownMessage, err := warp.NewUnsignedMessage(testNetworkID, chainID, unknownPayload.Bytes())
				require.NoError(t, err)
				return unknownMessage.Bytes(), nil
			},
			verifyStats: func(t *testing.T, stats *verifierStats) {
				require.EqualValues(t, 1, stats.messageParseFail.Snapshot().Count())
				require.EqualValues(t, 0, stats.blockValidationFail.Snapshot().Count())
			},
			errCode: ParseErrCode,
		},
	}

	for name, test := range tests {
		for _, withCache := range []bool{true, false} {
			testName := name
			if withCache {
				testName += "_with_cache"
			} else {
				testName += "_no_cache"
			}
			t.Run(testName, func(t *testing.T) {
				var sigCache cache.Cacher[ids.ID, []byte]
				if withCache {
					sigCache = lru.NewCache[ids.ID, []byte](100)
				} else {
					sigCache = &cache.Empty[ids.ID, []byte]{}
				}
				warpBackend, err := NewBackend(testNetworkID, chainID, warpSigner, warptest.EmptyBlockClient, database, sigCache, [][]byte{offchainMessage.Bytes()})
				require.NoError(t, err)
				handler := lp118.NewCachedHandler(sigCache, warpBackend, lp118Signer)

				requestBytes, expectedResponse := test.setup(warpBackend)
				// Use lp118 binary format for request
				reqBytes, err := lp118.MarshalSignatureRequest(&lp118.SignatureRequest{Message: requestBytes})
				require.NoError(t, err)

				responseBytes, handlerErr := handler.Request(context.Background(), ids.GenerateTestNodeID(), time.Time{}, reqBytes)
				if test.errCode != 0 {
					require.Error(t, handlerErr)
					// Check if it's a p2p.Error with the expected code
					if p2pErr, ok := handlerErr.(*p2p.Error); ok {
						require.Equal(t, test.errCode, p2pErr.Code)
					}
				} else {
					require.NoError(t, handlerErr)
				}

				test.verifyStats(t, warpBackend.(*backend).stats)

				// If the expected response is empty, assert that the handler returns an empty response and return early.
				if len(expectedResponse) == 0 {
					require.Len(t, responseBytes, 0, "expected response to be empty")
					return
				}
				// check cache is populated
				if withCache {
					require.NotZero(t, warpBackend.(*backend).signatureCache.Len())
				} else {
					require.Zero(t, warpBackend.(*backend).signatureCache.Len())
				}
				// Use lp118 binary format for response
				response, err := lp118.UnmarshalSignatureResponse(responseBytes)
				require.NoError(t, err, "error unmarshalling SignatureResponse")

				require.Equal(t, expectedResponse, response.Signature)
			})
		}
	}
}

func TestBlockSignatures(t *testing.T) {
	metricstest.WithMetrics(t)

	database := memdb.New()
	chainID := ids.GenerateTestID()

	// Create BLS key and signer
	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	warpSigner := warp.NewSigner(sk, testNetworkID, chainID)
	lp118Signer := &testWarpSigner{signer: warpSigner}

	knownBlkID := ids.GenerateTestID()
	blockClient := warptest.MakeBlockClient(knownBlkID)

	toMessageBytes := func(id ids.ID) []byte {
		idPayload, err := payload.NewHash(id[:])
		if err != nil {
			panic(err)
		}

		msg, err := warp.NewUnsignedMessage(testNetworkID, chainID, idPayload.Bytes())
		if err != nil {
			panic(err)
		}

		return msg.Bytes()
	}

	tests := map[string]struct {
		setup       func() (request []byte, expectedResponse []byte)
		verifyStats func(t *testing.T, stats *verifierStats)
		errCode     int32 // 0 means no error expected
	}{
		"known block": {
			setup: func() (request []byte, expectedResponse []byte) {
				hashPayload, err := payload.NewHash(knownBlkID[:])
				require.NoError(t, err)
				unsignedMessage, err := warp.NewUnsignedMessage(testNetworkID, chainID, hashPayload.Bytes())
				require.NoError(t, err)
				signature, err := warpSigner.Sign(unsignedMessage)
				require.NoError(t, err)
				return toMessageBytes(knownBlkID), signature[:]
			},
			verifyStats: func(t *testing.T, stats *verifierStats) {
				require.EqualValues(t, 0, stats.blockValidationFail.Snapshot().Count())
				require.EqualValues(t, 0, stats.messageParseFail.Snapshot().Count())
			},
		},
		"unknown block": {
			setup: func() (request []byte, expectedResponse []byte) {
				unknownBlockID := ids.GenerateTestID()
				return toMessageBytes(unknownBlockID), nil
			},
			verifyStats: func(t *testing.T, stats *verifierStats) {
				require.EqualValues(t, 1, stats.blockValidationFail.Snapshot().Count())
				require.EqualValues(t, 0, stats.messageParseFail.Snapshot().Count())
			},
			errCode: VerifyErrCode,
		},
	}

	for name, test := range tests {
		for _, withCache := range []bool{true, false} {
			testName := name
			if withCache {
				testName += "_with_cache"
			} else {
				testName += "_no_cache"
			}
			t.Run(testName, func(t *testing.T) {
				var sigCache cache.Cacher[ids.ID, []byte]
				if withCache {
					sigCache = lru.NewCache[ids.ID, []byte](100)
				} else {
					sigCache = &cache.Empty[ids.ID, []byte]{}
				}
				warpBackend, err := NewBackend(
					testNetworkID,
					chainID,
					warpSigner,
					blockClient,
					database,
					sigCache,
					nil,
				)
				require.NoError(t, err)
				handler := lp118.NewCachedHandler(sigCache, warpBackend, lp118Signer)

				requestBytes, expectedResponse := test.setup()
				// Use lp118 binary format for request
				reqBytes, err := lp118.MarshalSignatureRequest(&lp118.SignatureRequest{Message: requestBytes})
				require.NoError(t, err)

				responseBytes, handlerErr := handler.Request(context.Background(), ids.GenerateTestNodeID(), time.Time{}, reqBytes)
				if test.errCode != 0 {
					require.Error(t, handlerErr)
					// Check if it's a p2p.Error with the expected code
					if p2pErr, ok := handlerErr.(*p2p.Error); ok {
						require.Equal(t, test.errCode, p2pErr.Code)
					}
				} else {
					require.NoError(t, handlerErr)
				}

				test.verifyStats(t, warpBackend.(*backend).stats)

				// If the expected response is empty, assert that the handler returns an empty response and return early.
				if len(expectedResponse) == 0 {
					require.Len(t, responseBytes, 0, "expected response to be empty")
					return
				}
				// check cache is populated
				if withCache {
					require.NotZero(t, warpBackend.(*backend).signatureCache.Len())
				} else {
					require.Zero(t, warpBackend.(*backend).signatureCache.Len())
				}
				// Use lp118 binary format for response
				response, err := lp118.UnmarshalSignatureResponse(responseBytes)
				require.NoError(t, err, "error unmarshalling SignatureResponse")
				require.Equal(t, expectedResponse, response.Signature)
			})
		}
	}
}
