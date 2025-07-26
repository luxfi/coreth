// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"
)

const Version = "1.0.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("geth plugin %s\n", Version)
		os.Exit(0)
	}

	// Geth plugin is not needed for C-Chain - it's built into the node
	fmt.Println("This is a placeholder for the geth plugin. The C-Chain VM is built into luxd.")
	os.Exit(0)
}