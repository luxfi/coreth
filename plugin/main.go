// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/luxfi/log"
	"github.com/luxfi/vm/vms/rpcchainvm"
	"github.com/luxfi/vm/utils/ulimit"

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
		fmt.Printf("failed to set fd limit correctly due to: %s\n", err)
		os.Exit(1)
	}
	if err := rpcchainvm.Serve(context.Background(), log.NoLog{}, factory.NewPluginVM()); err != nil {
		fmt.Printf("failed to serve vm: %s\n", err)
		os.Exit(1)
	}
}
