// Copyright 2023 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package discover

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/luxfi/geth/metrics"
)

const (
	moduleName = "discover"
	// ingressMeterName is the prefix of the per-packet inbound metrics.
	ingressMeterName = moduleName + "/ingress"

	// egressMeterName is the prefix of the per-packet outbound metrics.
	egressMeterName = moduleName + "/egress"
)

var (
	bucketsCounter      []*metrics.Counter
	ingressTrafficMeter = metrics.NewRegisteredMeter(ingressMeterName, nil)
	egressTrafficMeter  = metrics.NewRegisteredMeter(egressMeterName, nil)
)

func init() {
	for i := 0; i < nBuckets; i++ {
		bucketsCounter = append(bucketsCounter, metrics.NewRegisteredCounter(fmt.Sprintf("%s/bucket/%d/count", moduleName, i), nil))
	}
}

// meteredUdpConn is a wrapper around a net.UDPConn that meters both the
// inbound and outbound network traffic.
type meteredUdpConn struct {
	udpConn UDPConn
}

func newMeteredConn(conn UDPConn) UDPConn {
	// Short circuit if metrics are disabled
	if !metrics.Enabled() {
		return conn
	}
	return &meteredUdpConn{udpConn: conn}
}

func (c *meteredUdpConn) Close() error {
	return c.udpConn.Close()
}

func (c *meteredUdpConn) LocalAddr() net.Addr {
	return c.udpConn.LocalAddr()
}

// ReadFromUDPAddrPort delegates a network read to the underlying connection, bumping the udp ingress traffic meter along the way.
func (c *meteredUdpConn) ReadFromUDPAddrPort(b []byte) (n int, addr netip.AddrPort, err error) {
	n, addr, err = c.udpConn.ReadFromUDPAddrPort(b)
	ingressTrafficMeter.Mark(int64(n))
	return n, addr, err
}

// WriteToUDPAddrPort delegates a network write to the underlying connection, bumping the udp egress traffic meter along the way.
func (c *meteredUdpConn) WriteToUDPAddrPort(b []byte, addr netip.AddrPort) (n int, err error) {
	n, err = c.udpConn.WriteToUDPAddrPort(b, addr)
	egressTrafficMeter.Mark(int64(n))
	return n, err
}
