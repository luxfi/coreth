// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"math/big"
	"sync"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/precompile/modules"
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/geth/common"
	ethparams "github.com/luxfi/geth/params"
)

// extraPayloads provides access to the extra chain config and rules payloads.
// Using geth's RegisterExtras pattern for Rules, and our own sync.Map for ChainConfig.
var extraPayloads = struct {
	ChainConfig *typedPayloadStore[*ethparams.ChainConfig, *extras.ChainConfig]
	Rules       ethparams.ExtraPayloads[*extras.ChainConfig, RulesExtra]
}{
	ChainConfig: &typedPayloadStore[*ethparams.ChainConfig, *extras.ChainConfig]{},
}

type typedPayloadStore[K comparable, V any] struct {
	store sync.Map
}

func (s *typedPayloadStore[K, V]) Get(key K) V {
	if v, ok := s.store.Load(key); ok {
		return v.(V)
	}
	var zero V
	return zero
}

func (s *typedPayloadStore[K, V]) Set(key K, value V) {
	s.store.Store(key, value)
}

// gethInit would ideally be a regular init() function, but it MUST be run
// before any calls to [params.ChainConfig.Rules]. See `config.go` for its call site.
func gethInit() any {
	// No-op - payloads are initialized above
	return nil
}

// init registers the extras with geth so that RulesExtra is populated
// when chainConfig.Rules() is called.
func init() {
	extraPayloads.Rules = ethparams.RegisterExtras(ethparams.Extras[*extras.ChainConfig, RulesExtra]{
		NewRules: func(c *ethparams.ChainConfig, r *ethparams.Rules, cEx *extras.ChainConfig, num *big.Int, isMerge bool, timestamp uint64) RulesExtra {
			return constructRulesExtra(c, r, cEx, num, isMerge, timestamp)
		},
	})
}

// constructRulesExtra acts as an adjunct to the [params.ChainConfig.Rules]
// method. Its primary purpose is to construct the extra payload for the
// [params.Rules] but it MAY also modify the [params.Rules].
func constructRulesExtra(c *ethparams.ChainConfig, r *ethparams.Rules, cEx *extras.ChainConfig, blockNum *big.Int, isMerge bool, timestamp uint64) RulesExtra {
	var rules RulesExtra
	if cEx == nil {
		return rules
	}
	rules.LuxRules = cEx.GetLuxRules(timestamp)

	// Initialize the stateful precompiles that should be enabled at [blockTimestamp].
	rules.Precompiles = make(map[common.Address]precompileconfig.Config)
	rules.Predicaters = make(map[common.Address]precompileconfig.Predicater)
	rules.AccepterPrecompiles = make(map[common.Address]precompileconfig.Accepter)
	for _, module := range modules.RegisteredModules() {
		if config := cEx.GetActivePrecompileConfig(module.Address, timestamp); config != nil && !config.IsDisabled() {
			rules.Precompiles[module.Address] = config
			if predicater, ok := config.(precompileconfig.Predicater); ok {
				rules.Predicaters[module.Address] = predicater
			}
			if precompileAccepter, ok := config.(precompileconfig.Accepter); ok {
				rules.AccepterPrecompiles[module.Address] = precompileAccepter
			}
		}
	}

	return rules
}

// GetRules returns the chain rules at the given block and timestamp,
// including Lux-specific rules in the extras.Rules.
// This is the preferred way to get rules when Lux-specific features are needed.
// The rules extra is stored so it can be retrieved via GetRulesExtra using ChainID as key.
func GetRules(c *ethparams.ChainConfig, blockNum *big.Int, isMerge bool, timestamp uint64) (ethparams.Rules, *extras.Rules) {
	r := c.Rules(blockNum, isMerge, timestamp)
	cEx := GetExtra(c)
	rulesEx := constructRulesExtra(c, &r, cEx, blockNum, isMerge, timestamp)
	// Store the rules extra so GetRulesExtra can find it using ChainID as key
	extra := (*extras.Rules)(&rulesEx)
	SetRulesExtra(&r, extra)
	return r, extra
}
