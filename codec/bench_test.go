// Copyright (c) 2012, 2013 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a BSD-style license found in the LICENSE file.

package codec

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// Sample way to run:
// go test -bi -bv -bd=1 -benchmem -bench Msgpack__Encode

var (
	_       = fmt.Printf
	benchTs *TestStruc

	approxSize int

	benchDoInitBench     bool
	benchVerify          bool
	benchUnscientificRes bool = false
	//depth of 0 maps to ~400bytes json-encoded string, 1 maps to ~1400 bytes, etc
	//For depth>1, we likely trigger stack growth for encoders, making benchmarking unreliable.
	benchDepth     int
	benchInitDebug bool
	benchCheckers  []benchChecker
)

type benchEncFn func(*TestStruc) ([]byte, error)
type benchDecFn func([]byte, *TestStruc) error
type benchChecker struct {
	name     string
	encodefn benchEncFn
	decodefn benchDecFn
}

func benchInitFlags() {
	flag.BoolVar(&benchInitDebug, "bdbg", false, "Bench Debug")
	flag.IntVar(&benchDepth, "bd", 1, "Bench Depth: If >1, potential unreliable results due to stack growth")
	flag.BoolVar(&benchDoInitBench, "bi", false, "Run Bench Init")
	flag.BoolVar(&benchVerify, "bv", false, "Verify Decoded Value during Benchmark")
	flag.BoolVar(&benchUnscientificRes, "bu", false, "Show Unscientific Results during Benchmark")
}

func benchInit() {	
	benchTs = newTestStruc(benchDepth, true)
	approxSize = approxDataSize(reflect.ValueOf(benchTs))
	bytesLen := 1024 * 4 * (benchDepth + 1) * (benchDepth + 1)
	if bytesLen < approxSize {
		bytesLen = approxSize
	}

	benchCheckers = append(benchCheckers,
		benchChecker{"msgpack", fnMsgpackEncodeFn, fnMsgpackDecodeFn},
		benchChecker{"binc", fnBincEncodeFn, fnBincDecodeFn},
		benchChecker{"gob", fnGobEncodeFn, fnGobDecodeFn},
		benchChecker{"json", fnJsonEncodeFn, fnJsonDecodeFn},
	)
	if benchDoInitBench {
		runBenchInit()
	}
}

func runBenchInit() {
	logT(nil, "..............................................")
	logT(nil, "BENCHMARK INIT: %v", time.Now())
	logT(nil, "To run full benchmark comparing encodings (MsgPack, Binc, JSON, GOB, etc), "+
		"use: \"go test -bench=.\"")
	logT(nil, "Benchmark: ")
	logT(nil, "\tStruct recursive Depth:             %d", benchDepth)
	if approxSize > 0 {
		logT(nil, "\tApproxDeepSize Of benchmark Struct: %d bytes", approxSize)
	}
	if benchUnscientificRes {
		logT(nil, "Benchmark One-Pass Run (with Unscientific Encode/Decode times): ")
	} else {
		logT(nil, "Benchmark One-Pass Run:")
	}
	for _, bc := range benchCheckers {
		doBenchCheck(bc.name, bc.encodefn, bc.decodefn)
	}
	logT(nil, "..............................................")
	if benchInitDebug {
		logT(nil, "<<<<====>>>> depth: %v, ts: %#v\n", benchDepth, benchTs)
	}
}

func doBenchCheck(name string, encfn benchEncFn, decfn benchDecFn) {
	runtime.GC()
	tnow := time.Now()
	buf, err := encfn(benchTs)
	if err != nil {
		logT(nil, "\t%10s: **** Error encoding benchTs: %v", name, err)
	}
	encDur := time.Now().Sub(tnow)
	encLen := len(buf)
	runtime.GC()
	if !benchUnscientificRes {
		logT(nil, "\t%10s: len: %d bytes\n", name, encLen)
		return
	}
	tnow = time.Now()
	if err = decfn(buf, new(TestStruc)); err != nil {
		logT(nil, "\t%10s: **** Error decoding into new TestStruc: %v", name, err)
	}
	decDur := time.Now().Sub(tnow)
	logT(nil, "\t%10s: len: %d bytes, encode: %v, decode: %v\n", name, encLen, encDur, decDur)
}

func fnBenchmarkEncode(b *testing.B, encName string, encfn benchEncFn) {
	runtime.GC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encfn(benchTs)
		if err != nil {
			logT(b, "Error encoding benchTs: %s: %v", encName, err)
			b.FailNow()
		}
	}
}

func fnBenchmarkDecode(b *testing.B, encName string, encfn benchEncFn, decfn benchDecFn) {
	buf, err := encfn(benchTs)
	if err != nil {
		logT(b, "Error encoding benchTs: %s: %v", encName, err)
		b.FailNow()
	}
	runtime.GC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts := new(TestStruc)
		if err = decfn(buf, ts); err != nil {
			logT(b, "Error decoding into new TestStruc: %s: %v", encName, err)
			b.FailNow()
		}
		if benchVerify {
			verifyTsTree(b, ts)
		}
	}
}

func verifyTsTree(b *testing.B, ts *TestStruc) {
	var ts0, ts1m, ts2m, ts1s, ts2s *TestStruc
	ts0 = ts

	if benchDepth > 0 {
		ts1m, ts1s = verifyCheckAndGet(b, ts0)
	}

	if benchDepth > 1 {
		ts2m, ts2s = verifyCheckAndGet(b, ts1m)
	}
	for _, tsx := range []*TestStruc{ts0, ts1m, ts2m, ts1s, ts2s} {
		if tsx != nil {
			verifyOneOne(b, tsx)
		}
	}
}

func verifyCheckAndGet(b *testing.B, ts0 *TestStruc) (ts1m *TestStruc, ts1s *TestStruc) {
	// if len(ts1m.Ms) <= 2 {
	// 	logT(b, "Error: ts1m.Ms len should be > 2. Got: %v", len(ts1m.Ms))
	// 	b.FailNow()
	// }
	if len(ts0.Its) == 0 {
		logT(b, "Error: ts0.Islice len should be > 0. Got: %v", len(ts0.Its))
		b.FailNow()
	}
	ts1m = ts0.Mtsptr["0"]
	ts1s = ts0.Its[0]
	if ts1m == nil || ts1s == nil {
		logT(b, "Error: At benchDepth 1, No *TestStruc found")
		b.FailNow()
	}
	return
}

func verifyOneOne(b *testing.B, ts *TestStruc) {
	if ts.I64slice[2] != int64(3) {
		logT(b, "Error: Decode failed by checking values")
		b.FailNow()
	}
}

func fnMsgpackEncodeFn(ts *TestStruc) (bs []byte, err error) {
	err = NewEncoderBytes(&bs, testMsgpackH).Encode(ts)
	return
}

func fnMsgpackDecodeFn(buf []byte, ts *TestStruc) error {
	return NewDecoderBytes(buf, testMsgpackH).Decode(ts)
}

func fnBincEncodeFn(ts *TestStruc) (bs []byte, err error) {
	err = NewEncoderBytes(&bs, testBincH).Encode(ts)
	return
}

func fnBincDecodeFn(buf []byte, ts *TestStruc) error {
	return NewDecoderBytes(buf, testBincH).Decode(ts)
}

func fnGobEncodeFn(ts *TestStruc) ([]byte, error) {
	bbuf := new(bytes.Buffer)
	err := gob.NewEncoder(bbuf).Encode(ts)
	return bbuf.Bytes(), err
}

func fnGobDecodeFn(buf []byte, ts *TestStruc) error {
	return gob.NewDecoder(bytes.NewBuffer(buf)).Decode(ts)
}

func fnJsonEncodeFn(ts *TestStruc) ([]byte, error) {
	return json.Marshal(ts)
}

func fnJsonDecodeFn(buf []byte, ts *TestStruc) error {
	return json.Unmarshal(buf, ts)
}

func Benchmark__Msgpack__Encode(b *testing.B) {
	fnBenchmarkEncode(b, "msgpack", fnMsgpackEncodeFn)
}

func Benchmark__Msgpack__Decode(b *testing.B) {
	fnBenchmarkDecode(b, "msgpack", fnMsgpackEncodeFn, fnMsgpackDecodeFn)
}

func Benchmark__Binc_____Encode(b *testing.B) {
	fnBenchmarkEncode(b, "binc", fnBincEncodeFn)
}

func Benchmark__Binc_____Decode(b *testing.B) {
	fnBenchmarkDecode(b, "binc", fnBincEncodeFn, fnBincDecodeFn)
}

func Benchmark__Gob______Encode(b *testing.B) {
	fnBenchmarkEncode(b, "gob", fnGobEncodeFn)
}

func Benchmark__Gob______Decode(b *testing.B) {
	fnBenchmarkDecode(b, "gob", fnGobEncodeFn, fnGobDecodeFn)
}

func Benchmark__Json_____Encode(b *testing.B) {
	fnBenchmarkEncode(b, "json", fnJsonEncodeFn)
}

func Benchmark__Json_____Decode(b *testing.B) {
	fnBenchmarkDecode(b, "json", fnJsonEncodeFn, fnJsonDecodeFn)
}
