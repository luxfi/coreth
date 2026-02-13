// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/luxfi/log"
	"github.com/luxfi/sys/ulimit"
	"github.com/luxfi/vm/rpc"

	"github.com/luxfi/coreth/plugin/evm"
	"github.com/luxfi/coreth/plugin/factory"
)

func main() {
	version, err := PrintVersion()
	if err != nil {
		fmt.Printf("couldn't get config: %s\n", err)
		os.Exit(1)
	}
	if version {
		fmt.Println(evm.Version)
		os.Exit(0)
	}
	if err := ulimit.Set(ulimit.DefaultFDLimit, log.NoLog{}); err != nil {
		fmt.Printf("failed to set fd limit: %s\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[EVM-PLUGIN] starting rpc.Serve (transport=%s)\n", os.Getenv("LUX_VM_TRANSPORT"))
	if err := rpc.Serve(context.Background(), log.NoLog{}, factory.NewPluginVM()); err != nil {
		fmt.Fprintf(os.Stderr, "[EVM-PLUGIN] rpc.Serve FAILED: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[EVM-PLUGIN] rpc.Serve returned nil - subprocess exiting normally\n")
}
