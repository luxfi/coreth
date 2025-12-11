// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package precompileconfig

// Upgrade contains the timestamp for the upgrade along with
// a boolean [Disable]. If [Disable] is set, the upgrade deactivates
// the precompile and clears its storage.
type Upgrade struct {
	BlockTimestamp *uint64 `json:"blockTimestamp"`
	Disable        bool    `json:"disable,omitempty"`
}

// Timestamp returns the timestamp this network upgrade goes into effect.
func (u *Upgrade) Timestamp() *uint64 {
	return u.BlockTimestamp
}

// IsDisabled returns true if the network upgrade deactivates the precompile.
func (u *Upgrade) IsDisabled() bool {
	return u.Disable
}

// Equal returns true iff [other] has the same blockTimestamp and has the
// same on value for the Disable flag.
func (u *Upgrade) Equal(other *Upgrade) bool {
	if other == nil {
		return false
	}
	return u.Disable == other.Disable && Uint64PtrEqual(u.BlockTimestamp, other.BlockTimestamp)
}

// Uint64PtrEqual returns true if both pointers are nil or both are non-nil and point to the same value
func Uint64PtrEqual(a, b *uint64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}