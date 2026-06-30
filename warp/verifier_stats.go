// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import "github.com/luxfi/geth/metrics"

type verifierStats struct {
	messageParseFail    *metrics.Counter
	blockValidationFail *metrics.Counter
}

func newVerifierStats() *verifierStats {
	s := &verifierStats{}
	s.messageParseFail = metrics.NewRegisteredCounter("warp_backend_message_parse_fail", nil)
	s.blockValidationFail = metrics.NewRegisteredCounter("warp_backend_block_validation_fail", nil)
	return s
}

func (h *verifierStats) IncBlockValidationFail() {
	h.blockValidationFail.Inc(1)
}

func (h *verifierStats) IncMessageParseFail() {
	h.messageParseFail.Inc(1)
}
