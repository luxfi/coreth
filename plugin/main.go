// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/luxfi/node/utils/logging"
	"github.com/luxfi/node/utils/ulimit"
	"github.com/luxfi/node/vms/rpcchainvm"

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
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		fmt.Printf("failed to set fd limit correctly due to: %s\n", err)
		os.Exit(1)
	}
	rpcchainvm.Serve(context.Background(), factory.NewPluginVM())
}
