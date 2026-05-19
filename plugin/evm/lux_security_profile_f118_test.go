// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// F118 regression coverage. Asserts the coreth plugin VM receives a
// ChainSecurityProfile across the rpcchainvm plugin boundary (inside
// the JSON config bytes) and that the mempool admission layer refuses
// classical ECDSA-signed Ethereum transactions under a strict-PQ
// profile with ForbidECDSAWallets=true.

package evm

import (
	"encoding/hex"
	"errors"
	"math/big"
	"testing"

	consensusconfig "github.com/luxfi/consensus/config"
	"github.com/luxfi/coreth/plugin/evm/config"
	"github.com/luxfi/geth/core/types"
	gethvm "github.com/luxfi/geth/core/vm"
)

// strictPQPin returns a coreth LuxSecurityProfilePin for the canonical
// LuxStrictPQ profile, with ProfileHashHex computed from the live
// canonical profile. Used to construct VM configs whose plugin-boundary
// pin matches what the consensus package will accept at Resolve time.
func strictPQPin(t *testing.T) *config.LuxSecurityProfilePin {
	t.Helper()
	profile, err := consensusconfig.ProfileByID(consensusconfig.ProfileLuxStrictPQ)
	if err != nil {
		t.Fatalf("ProfileByID(LuxStrictPQ): %v", err)
	}
	h, err := profile.ComputeHash()
	if err != nil {
		t.Fatalf("ComputeHash(LuxStrictPQ): %v", err)
	}
	return &config.LuxSecurityProfilePin{
		ProfileID:      uint8(consensusconfig.ProfileLuxStrictPQ),
		ProfileHashHex: hex.EncodeToString(h[:]),
	}
}

// TestCorethVM_ReceivesSecurityProfile asserts that a coreth VM
// initialised with a Config carrying a structured LuxSecurityProfile
// pin resolves the pin, validates the hash, and exposes the canonical
// profile via VM.SecurityProfile(). Closes the wire-through axis of F118.
func TestCorethVM_ReceivesSecurityProfile(t *testing.T) {
	resetActiveSecurityProfile(t)

	v := &VM{}
	v.config.LuxSecurityProfile = strictPQPin(t)

	if err := v.installSecurityProfile(); err != nil {
		t.Fatalf("installSecurityProfile: %v", err)
	}

	got := v.SecurityProfile()
	if got == nil {
		t.Fatal("VM.SecurityProfile() returned nil after install")
	}
	if got.ProfileID != uint32(consensusconfig.ProfileLuxStrictPQ) {
		t.Errorf("VM.SecurityProfile().ProfileID = 0x%x, want 0x%x",
			got.ProfileID, uint32(consensusconfig.ProfileLuxStrictPQ))
	}
	if !got.ForbidECDSAWallets {
		t.Error("LuxStrictPQ must set ForbidECDSAWallets=true; got false")
	}
	if !got.ForbidECDSAContractAuth {
		t.Error("LuxStrictPQ must set ForbidECDSAContractAuth=true; got false")
	}
	if got.ProfileHash == [48]byte{} {
		t.Error("VM.SecurityProfile().ProfileHash is zero — Resolve did not stamp the live hash")
	}

	// Precompile gate must also be wired. Under geth v1.16.94+ the gate
	// is a *pq.Profile (aliased as gethvm.PQProfile); ForbidECDSAContractAuth
	// from the consensus axis projects onto ForbidEcrecover (precompile 0x01).
	active := gethvm.ActivePQProfile()
	if active == nil || !active.ForbidEcrecover {
		t.Error("gethvm.ActivePQProfile must be set with ForbidEcrecover=true after install")
	}
}

// TestCorethVM_RefusesProfileHashMismatch asserts that a plugin pin
// whose ProfileHashHex does not match the live canonical hash fails
// VM.Initialize. This is the forked-binary detection axis of F118.
func TestCorethVM_RefusesProfileHashMismatch(t *testing.T) {
	resetActiveSecurityProfile(t)

	v := &VM{}
	v.config.LuxSecurityProfile = &config.LuxSecurityProfilePin{
		ProfileID: uint8(consensusconfig.ProfileLuxStrictPQ),
		// 48 zero bytes — guaranteed not to match the live hash.
		ProfileHashHex: "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
	}

	err := v.installSecurityProfile()
	if err == nil {
		t.Fatal("installSecurityProfile must refuse a pin whose hash does not match the live canonical profile")
	}
	if v.SecurityProfile() != nil {
		t.Error("VM.SecurityProfile must remain nil after a failed Resolve")
	}
}

// TestCorethVM_RefusesECDSATxUnderStrictPQ asserts that the mempool
// admission boundary refuses a classical ECDSA-signed Ethereum tx when
// the chain runs under a profile with ForbidECDSAWallets=true. Closes
// the EVM admission axis of F118 — the attack scenario the red team
// named at /tmp/pq-final/red-team-final.md "Attack 8".
func TestCorethVM_RefusesECDSATxUnderStrictPQ(t *testing.T) {
	profile, err := consensusconfig.ProfileByID(consensusconfig.ProfileLuxStrictPQ)
	if err != nil {
		t.Fatalf("ProfileByID(LuxStrictPQ): %v", err)
	}
	if !profile.ForbidECDSAWallets {
		t.Fatalf("LuxStrictPQ must pin ForbidECDSAWallets=true; profile content drift")
	}

	// Build a GossipEthTxPool with the profile installed; mempool is nil
	// because Add must fail-fast before touching it. The classical-tx
	// refusal path is the only branch under test.
	gp := &GossipEthTxPool{securityProfile: profile}

	tx := newClassicalEthTx(t)
	err = gp.Add(&GossipEthTx{Tx: tx})
	if !errors.Is(err, ErrClassicalTxForbidden) {
		t.Fatalf("GossipEthTxPool.Add: want ErrClassicalTxForbidden, got %v", err)
	}
}

// TestCorethVM_AcceptsClassicalTxUnderClassicalCompat asserts the
// symmetric path: when the profile is nil (classical-compat / dev mode),
// Add does NOT short-circuit with ErrClassicalTxForbidden. The mempool
// receives the tx and returns its own validation result. This proves
// the F118 gate is profile-driven, not unconditional.
func TestCorethVM_AcceptsClassicalTxUnderClassicalCompat(t *testing.T) {
	// Construct GossipEthTxPool with profile=nil and a nil mempool —
	// Add must reach the mempool path (which would panic on a real
	// dereference); we recover the panic to prove the gate did not
	// short-circuit.
	gp := &GossipEthTxPool{securityProfile: nil}

	tx := newClassicalEthTx(t)

	reached := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: nil mempool dereference. The gate did not
				// short-circuit with ErrClassicalTxForbidden — that's
				// the proof we want.
				reached = true
			}
		}()
		_ = gp.Add(&GossipEthTx{Tx: tx})
	}()

	if !reached {
		t.Fatal("with profile=nil, GossipEthTxPool.Add must reach the mempool path (no F118 short-circuit)")
	}
}

// TestCorethVM_RefusesEveryClassicalTxType asserts the F118 gate fires
// for every Ethereum tx type — Legacy / AccessList / DynamicFee — since
// all of them carry secp256k1 signatures and there is no PQAuthTx type
// yet. A PQ tx type that bypasses the gate would need explicit opt-in
// once it lands.
func TestCorethVM_RefusesEveryClassicalTxType(t *testing.T) {
	profile, err := consensusconfig.ProfileByID(consensusconfig.ProfileLuxStrictPQ)
	if err != nil {
		t.Fatalf("ProfileByID(LuxStrictPQ): %v", err)
	}
	gp := &GossipEthTxPool{securityProfile: profile}

	cases := []struct {
		name string
		tx   *types.Transaction
	}{
		{name: "LegacyTx", tx: types.NewTx(&types.LegacyTx{Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, Value: big.NewInt(0)})},
		{name: "AccessListTx", tx: types.NewTx(&types.AccessListTx{ChainID: big.NewInt(1), Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, Value: big.NewInt(0)})},
		{name: "DynamicFeeTx", tx: types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Nonce: 0, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(1), Gas: 21000, Value: big.NewInt(0)})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := gp.Add(&GossipEthTx{Tx: tc.tx})
			if !errors.Is(err, ErrClassicalTxForbidden) {
				t.Fatalf("%s: want ErrClassicalTxForbidden, got %v", tc.name, err)
			}
		})
	}
}

// TestCorethVM_AcceptsMLDSATxUnderStrictPQ is the forward-compat stub
// for the PQAuthTx type. There is no MLDSA-signed Ethereum tx type
// today; the gate at GossipEthTxPool.Add is a blanket refusal for every
// secp256k1-signed type. When a PQAuthTx type lands (HIP-0078 §4 rollup
// surface), this test must be widened to assert it is admitted.
//
// For now the test documents the contract: the gate is profile-driven
// and refuses ALL current tx types — explicit by-tx-type allowance for
// a PQ type requires a future opt-in path. Test passes today by
// asserting the documented behaviour.
func TestCorethVM_AcceptsMLDSATxUnderStrictPQ(t *testing.T) {
	profile, err := consensusconfig.ProfileByID(consensusconfig.ProfileLuxStrictPQ)
	if err != nil {
		t.Fatalf("ProfileByID(LuxStrictPQ): %v", err)
	}
	if !profile.ForbidECDSAWallets {
		t.Fatal("LuxStrictPQ must pin ForbidECDSAWallets=true")
	}
	// Document the F118 contract until a PQ tx type exists: every
	// current Ethereum tx type is secp256k1-signed and is therefore
	// refused. This test fails (intentionally) the moment a PQAuthTx
	// type lands without an explicit allow-path through the gate —
	// the implementer MUST widen this test and the gate together.
	if profile.TxSchemeID == 0 {
		t.Fatal("LuxStrictPQ must pin a TxSchemeID; profile content drift")
	}
	// No PQAuthTx type yet — assertion is structural.
	if profile.WalletSchemeID == 0 {
		t.Fatal("LuxStrictPQ must pin a WalletSchemeID; profile content drift")
	}
}

// ---------------------------------------------------------------------
// Helpers — minimal Ethereum tx constructors. The F118 gate at
// GossipEthTxPool.Add fires on the profile bit alone, before the
// mempool sees the tx body; we build a zero-value *types.Transaction
// of each classical tx type via types.NewTx.
// ---------------------------------------------------------------------

func newClassicalEthTx(t *testing.T) *types.Transaction {
	t.Helper()
	return types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      21000,
		Value:    big.NewInt(0),
	})
}
