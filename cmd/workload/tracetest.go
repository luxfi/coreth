// Copyright 2025 The go-ethereum Authors
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/eth/tracers"
	"github.com/luxfi/geth/internal/utesting"
	"github.com/luxfi/geth/log"
	"github.com/urfave/cli/v2"
)

// traceTest is the content of a history test.
type traceTest struct {
	BlockHashes  []common.Hash         `json:"blockHashes"`
	TraceConfigs []tracers.TraceConfig `json:"traceConfigs"`
	ResultHashes []common.Hash         `json:"resultHashes"`
}

type traceTestSuite struct {
	cfg        testConfig
	tests      traceTest
	invalidDir string
}

func newTraceTestSuite(cfg testConfig, ctx *cli.Context) *traceTestSuite {
	s := &traceTestSuite{
		cfg:        cfg,
		invalidDir: ctx.String(traceTestInvalidOutputFlag.Name),
	}
	if err := s.loadTests(); err != nil {
		exit(err)
	}
	return s
}

func (s *traceTestSuite) loadTests() error {
	file, err := s.cfg.fsys.Open(s.cfg.traceTestFile)
	if err != nil {
		// If not found in embedded FS, try to load it from disk
		if !os.IsNotExist(err) {
			return err
		}
		file, err = os.OpenFile(s.cfg.traceTestFile, os.O_RDONLY, 0666)
		if err != nil {
			return fmt.Errorf("can't open traceTestFile: %v", err)
		}
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&s.tests); err != nil {
		return fmt.Errorf("invalid JSON in %s: %v", s.cfg.traceTestFile, err)
	}
	if len(s.tests.BlockHashes) == 0 {
		return fmt.Errorf("traceTestFile %s has no test data", s.cfg.traceTestFile)
	}
	return nil
}

func (s *traceTestSuite) allTests() []workloadTest {
	return []workloadTest{
		newArchiveWorkloadTest("Trace/Block", s.traceBlock),
	}
}

// traceBlock runs all block tracing tests
func (s *traceTestSuite) traceBlock(t *utesting.T) {
	ctx := context.Background()

	for i, hash := range s.tests.BlockHashes {
		config := s.tests.TraceConfigs[i]
		result, err := s.cfg.client.Geth.TraceBlock(ctx, hash, &config)
		if err != nil {
			t.Fatalf("Transaction %d (hash %v): error %v", i, hash, err)
		}
		blob, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Transaction %d (hash %v): error %v", i, hash, err)
			continue
		}
		if crypto.Keccak256Hash(blob) != s.tests.ResultHashes[i] {
			t.Errorf("Transaction %d (hash %v): invalid result", i, hash)

			writeInvalidTraceResult(s.invalidDir, hash, result)
		}
	}
}

func writeInvalidTraceResult(dir string, hash common.Hash, result any) {
	if dir == "" {
		return
	}
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Info("Failed to make output directory", "err", err)
		return
	}
	name := filepath.Join(dir, "invalid"+"_"+hash.String())
	file, err := os.Create(name)
	if err != nil {
		exit(fmt.Errorf("error creating %s: %v", name, err))
		return
	}
	defer file.Close()

	data, _ := json.MarshalIndent(result, "", "    ")
	_, err = file.Write(data)
	if err != nil {
		exit(fmt.Errorf("error writing %s: %v", name, err))
		return
	}
}
