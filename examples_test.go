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
	"context"
	"fmt"
	"time"

	"github.com/johnknl/btkv"
)

type exampleRecord struct {
	ts   time.Time
	id   string
	data []byte
}

type bulkExampleRecords struct {
	records []exampleRecord
	idx     int
}

func (g *bulkExampleRecords) Ok() bool {
	return g.idx < len(g.records)
}

func (g *bulkExampleRecords) Values() (string, []byte, time.Time) {
	r := g.records[g.idx]
	g.idx++

	return r.id, r.data, r.ts
}

func Example() {
	ctx := context.Background()

	store := btkv.NewBoltTKV("example", "/tmp/btkv-example.db", true)

	if err := store.Open(time.Second); err != nil {
		panic(err)
	}

	defer store.Close() // nolint:errcheck // just an example

	now := time.Now()

	// Set the value of entity "a"
	existed, err := store.Set(
		[]byte(`{"id":"a"}`),
		now,
		"a",
	)

	fmt.Println(existed, err)

	// Get the value of id "a"
	val, err := store.Get("a")
	fmt.Printf("%s %v\n", val, err)

	// Bulk set some entities
	records := &bulkExampleRecords{
		records: []exampleRecord{
			{data: []byte(`{"id":"b"}`), ts: now.Add(-time.Minute), id: "b"},
			{data: []byte(`{"id":"c"}`), ts: now.Add(-2 * time.Minute), id: "c"},
			{data: []byte(`{"id":"d"}`), ts: now.Add(-3 * time.Minute), id: "d"},
			{data: []byte(`{"id":"e"}`), ts: now.Add(-4 * time.Hour), id: "e"},
		},
	}

	_ = store.BulkSet(ctx, records)

	// Get max 2 entities from a range (oldest first)
	from := now.Add(-3 * time.Minute)
	to := now.Add(-time.Minute)

	for data, err := range store.RangeValues(ctx, &from, &to, 0, 2) {
		fmt.Println("range: "+string(data), err)
	}

	// Delete entity "a"
	err = store.Delete("a")
	fmt.Println(err)

	// Output:
	// false <nil>
	// {"id":"a"} <nil>
	// range: {"id":"d"} <nil>
	// range: {"id":"c"} <nil>
	// <nil>
}
