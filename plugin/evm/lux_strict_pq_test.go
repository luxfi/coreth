// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/luxfi/geth/common"
	gethvm "github.com/luxfi/geth/core/vm"
)

// ecrecoverAddr is the canonical Ethereum precompile address for
// secp256k1 ecrecover, namely 0x0000000000000000000000000000000000000001.
var ecrecoverAddr = common.BytesToAddress([]byte{0x1})

// validEcrecoverInputHex is a well-known signature for ecrecover at 0x01.
// Mirrors the one in luxfi/geth/core/vm/lux_security_profile_test.go.
//
// The recovered address does NOT matter for these tests — what matters is
// that under strict-PQ the precompile returns ErrClassicalAuthForbidden
// and zero output bytes, regardless of input shape.
const validEcrecoverInputHex = "" +
	// hash
	"a35a39e7715a7b2c5d2e3a5d8e8f8a8b8c8d8e8f9091929394959697989a9b9c" +
	// v (left-padded; canonical v = 27 or 28)
	"000000000000000000000000000000000000000000000000000000000000001c" +
	// r
	"7b6d1f1f0a85b5a3aa3f0e57c8c30a1b1c1d1e1f2021222324252627282a2b2c" +
	// s
	"3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c"

// resetActiveSecurityProfile restores the package-level state so a strict-PQ
// test in this package does not leak across cases. The receiver is the
// geth vm package; the wrapper is identical in shape to the geth test
// helper so an audit reviewer can diff line-by-line.
func resetActiveSecurityProfile(t *testing.T) {
	t.Helper()
	prev := gethvm.ActiveSecurityProfile()
	t.Cleanup(func() {
		gethvm.SetActiveSecurityProfile(prev)
	})
}

// TestEcrecoverStrictPQ_Forbidden asserts the coreth-side wire-up: when
// Config.LuxStrictPQ is true, VM.Initialize installs the strict-PQ
// posture into the geth precompile layer, and the ecrecover precompile
// at 0x01 returns ErrClassicalAuthForbidden.
//
// This test exercises the gate by installing the strict-PQ profile
// directly (the same gate VM.Initialize fires) and then calling the
// precompile. The integration with VM.Initialize itself is exercised
// by the higher-level coreth integration tests; this focused test
// proves that under coreth's go.mod the gate IS callable and returns
// the expected error.
func TestEcrecoverStrictPQ_Forbidden(t *testing.T) {
	resetActiveSecurityProfile(t)
	gethvm.SetActiveSecurityProfile(&gethvm.LuxSecurityProfile{
		ForbidECDSAContractAuth: true,
	})

	contracts := gethvm.PrecompiledContractsByzantium
	ecrec, ok := contracts[ecrecoverAddr]
	if !ok {
		t.Fatal("ecrecover precompile not registered at 0x01")
	}

	input, err := hex.DecodeString(validEcrecoverInputHex)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}

	out, err := ecrec.Run(input)
	if !errors.Is(err, gethvm.ErrClassicalAuthForbidden) {
		t.Fatalf("ecrecover.Run: got out=%x err=%v; want ErrClassicalAuthForbidden",
			out, err)
	}
	if len(out) != 0 {
		t.Fatalf("strict-PQ ecrecover must return zero bytes, got %d", len(out))
	}
}

// TestEcrecoverClassicalCompat_Works asserts the classical-compat path
// is preserved under coreth: when no profile is installed (or the
// profile is permissive), ecrecover runs upstream go-ethereum semantics
// and does NOT return ErrClassicalAuthForbidden.
func TestEcrecoverClassicalCompat_Works(t *testing.T) {
	resetActiveSecurityProfile(t)
	gethvm.SetActiveSecurityProfile(nil)

	contracts := gethvm.PrecompiledContractsByzantium
	ecrec, ok := contracts[ecrecoverAddr]
	if !ok {
		t.Fatal("ecrecover precompile not registered at 0x01")
	}

	// Zero input — classical ecrecover returns (nil, nil) here. Only
	// the absence of ErrClassicalAuthForbidden is required.
	_, err := ecrec.Run(make([]byte, 128))
	if errors.Is(err, gethvm.ErrClassicalAuthForbidden) {
		t.Fatalf("classical-compat path must not return ErrClassicalAuthForbidden, got %v", err)
	}
}
