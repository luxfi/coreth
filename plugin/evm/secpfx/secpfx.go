// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package secpfx wires luxfi/utxo/secp256k1fx into the atomic-tx
// subsystem behind a small adapter. The upstream Fx machinery requires
// a host VM that exposes Logger / Clock / CodecRegistry — the registry
// is luxfi/codec-typed, which would otherwise leak the legacy codec
// dependency back into plugin/evm/atomic/.
//
// Keeping the glue here lets plugin/evm/atomic/* stay codec-free while
// preserving the structural Fx integration (semantic verification for
// atomic transfers).
package secpfx

import (
	"github.com/luxfi/codec"
	"github.com/luxfi/codec/linearcodec"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/log"
	"github.com/luxfi/timer/mockable"
	"github.com/luxfi/utxo/secp256k1fx"
)

// Host is the small surface the atomic-tx VM provides to the Fx. It
// matches the runtime-relevant subset of secp256k1fx.VM but lets us add
// our own registry without dragging luxfi/codec into the VM.
type Host interface {
	Clock() *mockable.Clock
	Logger() log.Logger
}

// fxVM adapts a [Host] to the upstream secp256k1fx.VM interface.
type fxVM struct {
	Host
	reg codec.Registry
}

func (v *fxVM) CodecRegistry() codec.Registry { return v.reg }

// Adapter is the lifecycle facade returned by [New]. It owns the Fx and
// the no-op registry, and offers exactly the three operations the
// atomic-tx VM needs to drive the Fx through its lifecycle plus the
// VerifyTransfer call used in semantic verification.
type Adapter struct {
	fx           *secp256k1fx.Fx
	recoverCache secp256k1.RecoverCacheType
}

// New constructs an Adapter wired to the given host. The fx is
// initialized eagerly so VerifyTransfer is callable as soon as New
// returns — atomic-tx accept/verify paths can rely on this.
func New(host Host) (*Adapter, error) {
	fx := &secp256k1fx.Fx{}
	if err := fx.Initialize(&fxVM{Host: host, reg: linearcodec.NewDefault()}); err != nil {
		return nil, err
	}
	return &Adapter{
		fx:           fx,
		recoverCache: secp256k1.NewRecoverCache(1024),
	}, nil
}

// Initialize is a no-op — the Adapter already initialized its wrapped
// Fx in [New]. It exists so the Adapter satisfies the luxfi/proto/p/fx.Fx
// interface that downstream verifiers depend on; calling Initialize a
// second time would re-run secp256k1fx.Fx.Initialize with the host VM
// reference, which is unsupported.
func (a *Adapter) Initialize(interface{}) error { return nil }

// Bootstrapping forwards to the underlying Fx.
func (a *Adapter) Bootstrapping() error { return a.fx.Bootstrapping() }

// Bootstrapped forwards to the underlying Fx.
func (a *Adapter) Bootstrapped() error { return a.fx.Bootstrapped() }

// VerifyTransfer is the only Fx call the atomic-tx verifier needs.
func (a *Adapter) VerifyTransfer(tx, in, cred, utxo interface{}) error {
	return a.fx.VerifyTransfer(tx, in, cred, utxo)
}

// VerifyPermission forwards to the underlying Fx. Required by the
// luxfi/proto/p/fx.Fx interface but not exercised by atomic-tx flows.
func (a *Adapter) VerifyPermission(tx, in, cred, controlGroup interface{}) error {
	return a.fx.VerifyPermission(tx, in, cred, controlGroup)
}

// CreateOutput forwards to the underlying Fx. Required by the
// luxfi/proto/p/fx.Fx interface but not exercised by atomic-tx flows.
func (a *Adapter) CreateOutput(amount uint64, controlGroup interface{}) (interface{}, error) {
	return a.fx.CreateOutput(amount, controlGroup)
}

// RecoverCache exposes the secp256k1 recover cache the host VM stashes
// next to the Adapter so transaction signature verification can reuse
// the cache between calls.
func (a *Adapter) RecoverCache() secp256k1.RecoverCacheType { return a.recoverCache }
