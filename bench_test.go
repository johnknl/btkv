// MIT License
//
// Copyright (C) 2025 John Kleijn
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE

package btkv_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/johnknl/btkv"
)

func newBenchStore(b *testing.B) *btkv.BoltTKV {
	b.Helper()

	store := btkv.NewBoltTKV(
		"bench",
		filepath.Join(b.TempDir(), "bench.db"),
		true,
	)

	if err := store.Open(time.Second); err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() {
		store.Close() // nolint:errcheck // ignore error on close
	})

	return store
}

func BenchmarkBoltTKV_SetN1(b *testing.B) {
	store := newBenchStore(b)

	data := []byte(`{"value":"hello world"}`)
	now := time.Now()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := "id-%d" + strconv.Itoa(i)

		if _, err := store.Set(data, now, id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBoltTKV_GetN1(b *testing.B) {
	store := newBenchStore(b)
	n := 10_000

	if err := store.BulkSet(b.Context(), newBenchGenerator(n, time.Now())); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := store.Get(fmt.Sprintf("id-%d", i%n)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBoltTKV_RangeValuesN1000(b *testing.B) {
	store := newBenchStore(b)
	now := time.Now()

	const n = 1_000
	if err := store.BulkSet(b.Context(), newBenchGenerator(n, now)); err != nil {
		b.Fatal(err)
	}

	ctx := b.Context()

	from := now.Add(10 * time.Second)
	to := now.Add(1000 * time.Second)

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for value, err := range store.RangeValues(ctx, &from, &to, 0, 1000) {
			if err != nil {
				b.Fatal(err)
			}

			_ = value
			count++
		}

		if count != 990 {
			b.Fatalf("expected 990 records, got %d", count)
		}
	}
}

type benchRecord struct { // nolint:govet
	ts   time.Time
	data []byte
	id   string
}

type benchGenerator struct {
	records []benchRecord
	idx     int
}

func (g *benchGenerator) reset() {
	g.idx = 0
}

func (g *benchGenerator) Ok() bool {
	return g.idx < len(g.records)
}

func (g *benchGenerator) Values() (id string, data []byte, ts time.Time) {
	r := g.records[g.idx]
	g.idx++
	return r.id, r.data, r.ts
}

func newBenchGenerator(n int, now time.Time) *benchGenerator {
	records := make([]benchRecord, n)

	for i := range records {
		records[i] = benchRecord{
			id:   fmt.Sprintf("id-%d", i),
			data: []byte(`{"value":"hello world"}`),
			ts:   now.Add(time.Duration(i) * time.Second),
		}
	}

	return &benchGenerator{
		records: records,
	}
}

func BenchmarkBoltTKV_BulkSetN100(b *testing.B) {
	ctx := b.Context()
	generator := newBenchGenerator(100, time.Now())

	b.ResetTimer()

	store := newBenchStore(b)

	for b.Loop() {
		err := store.BulkSet(ctx, generator)

		if err != nil {
			b.Fatal(err)
		}

		generator.reset()
	}
}
