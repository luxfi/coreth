// Copyright 2018 The go-ethereum Authors
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

package bn256

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	cloudflare "github.com/luxfi/geth/crypto/bn256/cloudflare"
	gnark "github.com/luxfi/geth/crypto/bn256/gnark"
	google "github.com/luxfi/geth/crypto/bn256/google"
)

func getG1Points(input io.Reader) (*cloudflare.G1, *google.G1, *gnark.G1) {
	_, xc, err := cloudflare.RandomG1(input)
	if err != nil {
		// insufficient input
		return nil, nil, nil
	}
	xg := new(google.G1)
	if _, err := xg.Unmarshal(xc.Marshal()); err != nil {
		panic(fmt.Sprintf("Could not marshal cloudflare -> google: %v", err))
	}
	xs := new(gnark.G1)
	if _, err := xs.Unmarshal(xc.Marshal()); err != nil {
		panic(fmt.Sprintf("Could not marshal cloudflare -> gnark: %v", err))
	}
	return xc, xg, xs
}

func getG2Points(input io.Reader) (*cloudflare.G2, *google.G2, *gnark.G2) {
	_, xc, err := cloudflare.RandomG2(input)
	if err != nil {
		// insufficient input
		return nil, nil, nil
	}
	xg := new(google.G2)
	if _, err := xg.Unmarshal(xc.Marshal()); err != nil {
		panic(fmt.Sprintf("Could not marshal cloudflare -> google: %v", err))
	}
	xs := new(gnark.G2)
	if _, err := xs.Unmarshal(xc.Marshal()); err != nil {
		panic(fmt.Sprintf("Could not marshal cloudflare -> gnark: %v", err))
	}
	return xc, xg, xs
}

// fuzzAdd fuzzes bn256 addition between the Google, Cloudflare and Gnark libraries.
func fuzzAdd(data []byte) int {
	input := bytes.NewReader(data)
	xc, xg, xs := getG1Points(input)
	if xc == nil {
		return 0
	}
	yc, yg, ys := getG1Points(input)
	if yc == nil {
		return 0
	}
	// Ensure libs can parse the second curve point
	// Add the two points and ensure they result in the same output
	rc := new(cloudflare.G1)
	rc.Add(xc, yc)

	rg := new(google.G1)
	rg.Add(xg, yg)

	rs := new(gnark.G1)
	rs.Add(xs, ys)

	if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
		panic("add mismatch: cloudflare/google")
	}

	if !bytes.Equal(rc.Marshal(), rs.Marshal()) {
		panic("add mismatch: cloudflare/gnark")
	}
	return 1
}

// fuzzMul fuzzes bn256 scalar multiplication between the Google, Cloudflare
// and Gnark libraries.
func fuzzMul(data []byte) int {
	input := bytes.NewReader(data)
	pc, pg, ps := getG1Points(input)
	if pc == nil {
		return 0
	}
	// Add the two points and ensure they result in the same output
	remaining := input.Len()
	if remaining == 0 {
		return 0
	}
	if remaining > 128 {
		// The evm only ever uses 32 byte integers, we need to cap this otherwise
		// we run into slow exec. A 236Kb byte integer cause oss-fuzz to report it as slow.
		// 128 bytes should be fine though
		return 0
	}
	buf := make([]byte, remaining)
	input.Read(buf)

	rc := new(cloudflare.G1)
	rc.ScalarMult(pc, new(big.Int).SetBytes(buf))

	rg := new(google.G1)
	rg.ScalarMult(pg, new(big.Int).SetBytes(buf))

	rs := new(gnark.G1)
	rs.ScalarMult(ps, new(big.Int).SetBytes(buf))

	if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
		panic("scalar mul mismatch: cloudflare/google")
	}
	if !bytes.Equal(rc.Marshal(), rs.Marshal()) {
		panic("scalar mul mismatch: cloudflare/gnark")
	}
	return 1
}

func fuzzPair(data []byte) int {
	input := bytes.NewReader(data)
	pc, pg, ps := getG1Points(input)
	if pc == nil {
		return 0
	}
	tc, tg, ts := getG2Points(input)
	if tc == nil {
		return 0
	}

	// Pair the two points and ensure they result in the same output
	clPair := cloudflare.Pair(pc, tc).Marshal()
	gPair := google.Pair(pg, tg).Marshal()
	sPair := gnark.Pair(ps, ts).Marshal()

	if !bytes.Equal(clPair, gPair) {
		panic("pairing mismatch: cloudflare/google")
	}

	normalizedClPair := normalizeGTToGnark(clPair).Marshal()
	if !bytes.Equal(normalizedClPair, sPair) {
		panic("pairing mismatch: cloudflare/gnark")
	}

	return 1
}

func fuzzUnmarshalG1(input []byte) int {
	rc := new(cloudflare.G1)
	_, errC := rc.Unmarshal(input)

	rg := new(google.G1)
	_, errG := rg.Unmarshal(input)

	rs := new(gnark.G1)
	_, errS := rs.Unmarshal(input)

	if errC != nil && errG != nil && errS != nil {
		return 0 // bad input
	}
	if errC == nil && errG == nil && errS == nil {
		//make sure we unmarshalled the same points:
		if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
			panic("marshaling mismatch: cloudflare/google")
		}
		if !bytes.Equal(rc.Marshal(), rs.Marshal()) {
			panic("marshaling mismatch: cloudflare/gnark")
		}
		return 1
	} else {
		panic(fmt.Sprintf("error missmatch: cf: %v g: %v gn: %v", errC, errG, errS))
	}
}

func fuzzUnmarshalG2(input []byte) int {
	rc := new(cloudflare.G2)
	_, errC := rc.Unmarshal(input)

	rg := new(google.G2)
	_, errG := rg.Unmarshal(input)

	rs := new(gnark.G2)
	_, errS := rs.Unmarshal(input)

	if errC != nil && errG != nil && errS != nil {
		return 0 // bad input
	}
	if errC == nil && errG == nil && errS == nil {
		//make sure we unmarshalled the same points:
		if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
			panic("marshaling mismatch: cloudflare/google")
		}
		if !bytes.Equal(rc.Marshal(), rs.Marshal()) {
			panic("marshaling mismatch: cloudflare/gnark")
		}
		return 1
	} else {
		panic(fmt.Sprintf("error missmatch: cf: %v g: %v gn: %v", errC, errG, errS))
	}
}

// normalizeGTToGnark scales a Cloudflare/Google GT element by `s`
// so that it can be compared with a gnark GT point.
//
// For the definition of `s` see 3.5 in https://eprint.iacr.org/2015/192.pdf
func normalizeGTToGnark(cloudflareOrGoogleGT []byte) *gnark.GT {
	// Compute s = 2*u(6*u^2 + 3*u + 1)
	u, _ := new(big.Int).SetString("0x44e992b44a6909f1", 0)
	u_exp2 := new(big.Int).Exp(u, big.NewInt(2), nil)   // u^2
	u_6_exp2 := new(big.Int).Mul(big.NewInt(6), u_exp2) // 6*u^2
	u_3 := new(big.Int).Mul(big.NewInt(3), u)           // 3*u
	inner := u_6_exp2.Add(u_6_exp2, u_3)                // 6*u^2 + 3*u
	inner.Add(inner, big.NewInt(1))                     // 6*u^2 + 3*u + 1
	u_2 := new(big.Int).Mul(big.NewInt(2), u)           // 2*u
	s := u_2.Mul(u_2, inner)                            // 2*u(6*u^2 + 3*u + 1)

	// Scale the Cloudflare/Google GT element by `s`
	gRes := new(gnark.GT)
	if err := gRes.Unmarshal(cloudflareOrGoogleGT); err != nil {
		panic(err)
	}
	gRes = gRes.Exp(*gRes, s)

	return gRes
}
