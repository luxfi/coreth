// Copyright 2020 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"fmt"

	"github.com/luxfi/geth/eth/ethconfig"
	"github.com/luxfi/geth/internal/flags"
	"github.com/urfave/cli/v2"
)

var ShowDeprecated = &cli.Command{
	Action:      showDeprecated,
	Name:        "show-deprecated-flags",
	Usage:       "Show flags that have been deprecated",
	ArgsUsage:   " ",
	Description: "Show flags that have been deprecated and will soon be removed",
}

var DeprecatedFlags = []cli.Flag{
	NoUSBFlag,
	LegacyWhitelistFlag,
	CacheTrieJournalFlag,
	CacheTrieRejournalFlag,
	LegacyDiscoveryV5Flag,
	TxLookupLimitFlag,
	LogBacktraceAtFlag,
	LogDebugFlag,
	MinerNewPayloadTimeoutFlag,
	MinerEtherbaseFlag,
	MiningEnabledFlag,
	MetricsEnabledExpensiveFlag,
	EnablePersonal,
	UnlockedAccountFlag,
	InsecureUnlockAllowedFlag,
}

var (
	// Deprecated May 2020, shown in aliased flags section
	NoUSBFlag = &cli.BoolFlag{
		Name:     "nousb",
		Hidden:   true,
		Usage:    "Disables monitoring for and managing USB hardware wallets (deprecated)",
		Category: flags.DeprecatedCategory,
	}
	// Deprecated March 2022
	LegacyWhitelistFlag = &cli.StringFlag{
		Name:     "whitelist",
		Hidden:   true,
		Usage:    "Comma separated block number-to-hash mappings to enforce (<number>=<hash>) (deprecated in favor of --eth.requiredblocks)",
		Category: flags.DeprecatedCategory,
	}
	// Deprecated July 2023
	CacheTrieJournalFlag = &cli.StringFlag{
		Name:     "cache.trie.journal",
		Hidden:   true,
		Usage:    "Disk journal directory for trie cache to survive node restarts",
		Category: flags.DeprecatedCategory,
	}
	CacheTrieRejournalFlag = &cli.DurationFlag{
		Name:     "cache.trie.rejournal",
		Hidden:   true,
		Usage:    "Time interval to regenerate the trie cache journal",
		Category: flags.DeprecatedCategory,
	}
	LegacyDiscoveryV5Flag = &cli.BoolFlag{
		Name:     "v5disc",
		Hidden:   true,
		Usage:    "Enables the experimental RLPx V5 (Topic Discovery) mechanism (deprecated, use --discv5 instead)",
		Category: flags.DeprecatedCategory,
	}
	// Deprecated August 2023
	TxLookupLimitFlag = &cli.Uint64Flag{
		Name:     "txlookuplimit",
		Hidden:   true,
		Usage:    "Number of recent blocks to maintain transactions index for (default = about one year, 0 = entire chain) (deprecated, use history.transactions instead)",
		Value:    ethconfig.Defaults.TransactionHistory,
		Category: flags.DeprecatedCategory,
	}
	// Deprecated November 2023
	LogBacktraceAtFlag = &cli.StringFlag{
		Name:     "log.backtrace",
		Hidden:   true,
		Usage:    "Request a stack trace at a specific logging statement (deprecated)",
		Value:    "",
		Category: flags.DeprecatedCategory,
	}
	LogDebugFlag = &cli.BoolFlag{
		Name:     "log.debug",
		Hidden:   true,
		Usage:    "Prepends log messages with call-site location (deprecated)",
		Category: flags.DeprecatedCategory,
	}
	// Deprecated February 2024
	MinerNewPayloadTimeoutFlag = &cli.DurationFlag{
		Name:     "miner.newpayload-timeout",
		Hidden:   true,
		Usage:    "Specify the maximum time allowance for creating a new payload (deprecated)",
		Value:    ethconfig.Defaults.Miner.Recommit,
		Category: flags.DeprecatedCategory,
	}
	MinerEtherbaseFlag = &cli.StringFlag{
		Name:     "miner.etherbase",
		Hidden:   true,
		Usage:    "0x prefixed public address for block mining rewards (deprecated)",
		Category: flags.DeprecatedCategory,
	}
	MiningEnabledFlag = &cli.BoolFlag{
		Name:     "mine",
		Hidden:   true,
		Usage:    "Enable mining (deprecated)",
		Category: flags.DeprecatedCategory,
	}
	MetricsEnabledExpensiveFlag = &cli.BoolFlag{
		Name:     "metrics.expensive",
		Hidden:   true,
		Usage:    "Enable expensive metrics collection and reporting (deprecated)",
		Category: flags.DeprecatedCategory,
	}
	// Deprecated Oct 2024
	EnablePersonal = &cli.BoolFlag{
		Name:     "rpc.enabledeprecatedpersonal",
		Hidden:   true,
		Usage:    "This used to enable the 'personal' namespace.",
		Category: flags.DeprecatedCategory,
	}
	UnlockedAccountFlag = &cli.StringFlag{
		Name:     "unlock",
		Hidden:   true,
		Usage:    "Comma separated list of accounts to unlock (deprecated)",
		Value:    "",
		Category: flags.DeprecatedCategory,
	}
	InsecureUnlockAllowedFlag = &cli.BoolFlag{
		Name:     "allow-insecure-unlock",
		Hidden:   true,
		Usage:    "Allow insecure account unlocking when account-related RPCs are exposed by http (deprecated)",
		Category: flags.DeprecatedCategory,
	}
)

// showDeprecated displays deprecated flags that will be soon removed from the codebase.
func showDeprecated(*cli.Context) error {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Println("The following flags are deprecated and will be removed in the future!")
	fmt.Println("--------------------------------------------------------------------")
	fmt.Println()
	for _, flag := range DeprecatedFlags {
		fmt.Println(flag.String())
	}
	fmt.Println()
	return nil
}
