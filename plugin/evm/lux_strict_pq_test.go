// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"errors"
	"testing"

	"github.com/luxfi/geth/common"
	gethvm "github.com/luxfi/geth/core/vm"
)

// ecrecoverAddr is the canonical Ethereum precompile address for
// secp256k1 ecrecover, namely 0x0000000000000000000000000000000000000001.
var ecrecoverAddr = common.BytesToAddress([]byte{0x1})

// resetActiveSecurityProfile restores the deprecated package-global PQ
// projection between tests in this package. The authoritative gate is the
// chain-config field gethvm.ChainConfig.PQ exercised by
// (*EVM).runPrecompile; the package-global is retained as a back-compat
// shim that delegates to pq.SetActive. Resetting it isolates tests that
// would otherwise inherit state from prior cases.
func resetActiveSecurityProfile(t *testing.T) {
	t.Helper()
	prev := gethvm.ActivePQProfile()
	t.Cleanup(func() {
		gethvm.SetPQProfile(prev)
	})
}

// TestEcrecoverStrictPQ_Forbidden asserts the strict-PQ gate refuses
// the ecrecover precompile (op 0x01). Under geth v1.16.94+ the gate is
// exposed via *PQProfile.RefuseUnder(Op); the per-precompile Run() no
// longer gates by itself — the EVM dispatches via runPrecompile which
// invokes RefuseUnder.
func TestEcrecoverStrictPQ_Forbidden(t *testing.T) {
	profile := &gethvm.PQProfile{ForbidEcrecover: true}
	err := profile.RefuseUnder(gethvm.OpEcrecover)
	if !errors.Is(err, gethvm.ErrEcrecoverForbidden) {
		t.Fatalf("RefuseUnder(OpEcrecover): got err=%v; want ErrEcrecoverForbidden", err)
	}
}

// TestEcrecoverClassicalCompat_Works asserts that a nil profile (and a
// permissive profile) admit ecrecover. This preserves the classical-compat
// guarantee: chains without a strict-PQ pin behave identically to upstream
// go-ethereum.
func TestEcrecoverClassicalCompat_Works(t *testing.T) {
	var nilProfile *gethvm.PQProfile
	if err := nilProfile.RefuseUnder(gethvm.OpEcrecover); err != nil {
		t.Fatalf("nil profile must admit ecrecover; got %v", err)
	}

	permissive := &gethvm.PQProfile{}
	if err := permissive.RefuseUnder(gethvm.OpEcrecover); err != nil {
		t.Fatalf("permissive profile must admit ecrecover; got %v", err)
	}
}
