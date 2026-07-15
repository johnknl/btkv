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
	"path/filepath"
	"testing"
	"time"

	"github.com/johnknl/btkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bulkGenerator struct {
	records []record
	idx     int
}

type record struct {
	lastModified time.Time
	id           string
	data         []byte
}

func (g *bulkGenerator) Ok() bool {
	return g.idx < len(g.records)
}

func (g *bulkGenerator) Values() (string, []byte, time.Time) {
	r := g.records[g.idx]
	g.idx++

	return r.id, r.data, r.lastModified
}

func TestBoltTKV_CRUD(t *testing.T) {
	ctx := context.Background()

	storePath := filepath.Join(t.TempDir(), "test.db")
	store := btkv.NewBoltTKV(t.Name(), storePath, true)

	require.NoError(t, store.Open(time.Second))

	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now()
	id := "a"
	data := []byte(`{"id":"a"}`)
	lastModified := now

	t.Run("Set", func(t *testing.T) {
		existed, err := store.Set(data, lastModified, id)

		require.NoError(t, err)
		assert.False(t, existed)

		exists, err := store.Exists(id)

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Get", func(t *testing.T) {
		foundData, err := store.Get(id)

		require.NoError(t, err)
		assert.Equal(t, data, foundData)
	})

	t.Run("BulkSet", func(t *testing.T) {
		require.NoError(t, store.BulkSet(ctx, &bulkGenerator{}))

		gen := &bulkGenerator{
			records: []record{
				{
					id:           "a",
					data:         []byte(`{"id":"b"}`),
					lastModified: now.Add(-time.Minute),
				},
				{
					id:           "b",
					data:         []byte(`{"id":"c"}`),
					lastModified: now.Add(-2 * time.Minute),
				},
				{
					id:           "c",
					data:         []byte(`{"id":"d"}`),
					lastModified: now.Add(-3 * time.Minute),
				},
				{
					id:           "d",
					data:         []byte(`{"id":"e"}`),
					lastModified: now.Add(-4 * time.Hour),
				},
			},
		}

		require.NoError(t, store.BulkSet(ctx, gen))
	})

	t.Run("Range", func(t *testing.T) {
		var found [][]byte

		for value, err := range store.RangeValues(ctx, nil, nil, 0, 0) {
			require.NoError(t, err)
			found = append(found, value)
		}

		assert.NotEmpty(t, found)
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(id)

		require.NoError(t, err)

		exists, err := store.Exists(id)

		require.NoError(t, err)
		assert.False(t, exists)
	})
}
