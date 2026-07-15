# btkv

Wrapper around [bbolt](https://github.com/etcd-io/bbolt) for time-ordered, k/v data.

## Usage

```go
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
		fmt.Println(string(data), err)
	}

	// Delete entity "a"
	err = store.Delete("a")
	fmt.Println(err)

	// Output:
	// false <nil>
	// {"id":"a"} <nil>
	// {"id":"d"} <nil>
	// {"id":"c"} <nil>
	// <nil>
}
```




## Benchmarks

`btkv` is read optimized, specifically time range optimized. In the below benchmark `RangeValues()` is used to
yield `990` values. That is ~60 ns per record. Write operations are dominated by `fsync`: it doesn't matter much
if you write `1` or `100` records. Batching writes is highly recommended.

```
goos: linux
goarch: amd64
pkg: github.com/johnknl/btkv
cpu: AMD Ryzen 9 5950X 16-Core Processor
BenchmarkBoltTKV_SetN1
BenchmarkBoltTKV_SetN1-32                           1305            937898 ns/op           38644 B/op         85 allocs/op
BenchmarkBoltTKV_SetN1-32                           1286            935848 ns/op           38670 B/op         85 allocs/op
BenchmarkBoltTKV_SetN1-32                           1306            921649 ns/op           38669 B/op         85 allocs/op
BenchmarkBoltTKV_GetN1
BenchmarkBoltTKV_GetN1-32                         623986              1723 ns/op             777 B/op         15 allocs/op
BenchmarkBoltTKV_GetN1-32                         634867              1775 ns/op             777 B/op         15 allocs/op
BenchmarkBoltTKV_GetN1-32                         823404              1734 ns/op             777 B/op         15 allocs/op
BenchmarkBoltTKV_RangeValuesN1000
BenchmarkBoltTKV_RangeValuesN1000-32               19045             62252 ns/op           24304 B/op       1001 allocs/op
BenchmarkBoltTKV_RangeValuesN1000-32               19462             60632 ns/op           24304 B/op       1001 allocs/op
BenchmarkBoltTKV_RangeValuesN1000-32               20029             60161 ns/op           24304 B/op       1001 allocs/op
BenchmarkBoltTKV_BulkSetN100
BenchmarkBoltTKV_BulkSetN100-32                     1207            979246 ns/op           45457 B/op       1071 allocs/op
BenchmarkBoltTKV_BulkSetN100-32                     1068            974017 ns/op           45462 B/op       1071 allocs/op
BenchmarkBoltTKV_BulkSetN100-32                     1237            973808 ns/op           45433 B/op       1071 allocs/op
PASS
ok      github.com/johnknl/btkv 15.428s
```

## Documentation / Usage

Documentation and usage examples are available on [pkg.go.dev](https://pkg.go.dev/github.com/johnknl/btkv).
