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

package btkv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"path"
	"time"
	"unsafe"

	bolt "go.etcd.io/bbolt"
)

var (
	// ErrNothingToDelete is returned when trying to delete a non-existing entity.
	ErrNothingToDelete = errors.New("nothing to delete")

	emptyKey Key
)

// BoltTKV is a k/v store backed by Bolt.
// It uses a sorted set to keep track of last
// modified time and enable range queries.
type BoltTKV struct {
	db          *bolt.DB
	path        string
	namespace   string
	mainBucket  []byte
	indexBucket []byte
	lastKey     Key
	writable    bool
}

// NewBoltTKV creates a new BoltTKV instance.
// The namespace is used as bucket name in Bolt.
func NewBoltTKV(namespace, filePath string, writable bool) *BoltTKV {
	return &BoltTKV{
		path:      filePath,
		namespace: namespace,
		writable:  writable,
	}
}

// Open the underlying database
func (r *BoltTKV) Open(timeout time.Duration) error {
	dirName := path.Dir(r.path)
	if _, err := os.Stat(dirName); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed creating dir for bolt db: %w", err)
		}

		if err := os.MkdirAll(dirName, 0750); err != nil {
			return fmt.Errorf("failed creating dir for bolt db: %w", err)
		}
	}

	var err error
	if r.db, err = bolt.Open(r.path, 0600, &bolt.Options{
		Timeout:  timeout,
		ReadOnly: !r.writable,
	}); err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err = r.initialize(); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	return nil
}

// Close the underlying database
func (r *BoltTKV) Close() error {
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			return fmt.Errorf("close database: %w", err)
		}
	}

	return nil
}

func (r *BoltTKV) initialize() (err error) {
	if !r.writable {
		return nil
	}

	tx, err := r.db.Begin(true)
	if err != nil {
		return err
	}

	r.mainBucket = s2b(r.namespace)
	r.indexBucket = append(s2b(r.namespace), s2b(".Index")...)

	mainBucket, err := tx.CreateBucketIfNotExists(r.mainBucket)
	if err != nil {
		return err
	}

	if _, err := tx.CreateBucketIfNotExists(r.indexBucket); err != nil {
		return err
	}

	last, _ := mainBucket.Cursor().Last()
	r.lastKey = NewKey(last)

	return tx.Commit()
}

// At returns the value of an entity at a given key.
func (r *BoltTKV) At(key Key) (b []byte, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		main := r.bucket(tx)
		b = cp(main.Get(key.Bytes()))
		return nil
	})

	return
}

// Get an entity by ID.
func (r *BoltTKV) Get(id string) (b []byte, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		main, index := r.buckets(tx)
		b = cp(main.Get(index.Get(s2b(id))))
		return nil
	})

	if err != nil {
		err = fmt.Errorf("btkv view: %w", err)
	}

	return
}

type generator interface {
	// Ok returns true if there is a next entity to generate.
	Ok() bool

	// Values returns the next entity's ID, data and last modified time.
	Values() (id string, data []byte, lastModified time.Time)
}

// BulkSet sets multiple entities in the store.
func (r *BoltTKV) BulkSet(ctx context.Context, generator generator) (err error) {
	lastKey := r.lastKey // store the last key before the bulk set operation

	if err = r.db.Update(func(tx *bolt.Tx) error {
		for generator.Ok() {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			id, data, ts := generator.Values()

			var key Key
			key, _, err = r.put(tx, id, data, ts, lastKey)
			if err != nil {
				return fmt.Errorf("btkv put: %w", err)
			}

			lastKey = key
		}

		return nil
	}); err != nil {
		return fmt.Errorf("btkv bulkset: %w", err)
	}

	r.lastKey = lastKey // update the last key after the bulk set operation

	return nil
}

func (r *BoltTKV) put(tx *bolt.Tx, id string, data []byte, lastModified time.Time, lastKey Key) (Key, bool, error) {
	main, idxBkt := r.buckets(tx)
	key := MarshalTime(lastModified, lastKey)

	var exists bool
	oldKey := idxBkt.Get(s2b(id))

	if oldKey != nil {
		exists = true
		if err := main.Delete(oldKey); err != nil {
			return emptyKey, false, err
		}
	}

	if err := main.Put(key.Bytes(), data); err != nil {
		return emptyKey, false, err
	}

	if err := idxBkt.Put(s2b(id), key.Bytes()); err != nil {
		return emptyKey, false, err
	}

	return key, exists, nil
}

// Set an entity in the store by ID.
// If the entity already exists, it will be overwritten.
// Returns boolean true if entity already existed.
func (r *BoltTKV) Set(data []byte, lastModified time.Time, id string) (exists bool, err error) {
	var lastKey Key
	if err = r.db.Update(func(tx *bolt.Tx) error {
		lastKey, exists, err = r.put(tx, id, data, lastModified, r.lastKey)

		return err
	}); err != nil {
		return false, fmt.Errorf("btkv update: %w", err)
	}

	r.lastKey = lastKey

	return
}

// Exists checks if an entity exists in the store by ID.
func (r *BoltTKV) Exists(id string) (exists bool, err error) {
	if err = r.db.View(func(tx *bolt.Tx) error {
		idxBkt := r.index(tx)
		exists = idxBkt.Get(s2b(id)) != nil

		return nil
	}); err != nil {
		return false, fmt.Errorf("btkv exists: %w", err)
	}

	return
}

// Delete an entity from the store by ID.
func (r *BoltTKV) Delete(id string) (err error) {
	err = r.db.Update(func(tx *bolt.Tx) error {
		main, idxBkt := r.buckets(tx)
		idBytes := s2b(id)
		key := idxBkt.Get(idBytes)
		if key == nil {
			return ErrNothingToDelete
		}

		if err = idxBkt.Delete(idBytes); err != nil {
			return err
		}

		return main.Delete(key)
	})

	if err != nil {
		err = fmt.Errorf("btkv delete: %w", err)
	}

	return
}

// RangeKeys returns an iterator over the keys in the store between the given time range.
// The iterator yields keys in ascending order of their timestamps.
// Note that `to` is exclusive.
func (r *BoltTKV) RangeKeys(
	ctx context.Context,
	from, to *time.Time,
	offset, limit int,
) iter.Seq2[Key, error] {
	return func(yield func(Key, error) bool) {
		var idx int
		err := r.db.View(func(tx *bolt.Tx) error {
			c := r.bucket(tx).Cursor()

			var start, end []byte
			if from != nil {
				start = MarshalTime(*from, emptyKey).Bytes()
			}

			if to != nil {
				end = MarshalTime(*to, emptyKey).Bytes()
			}

			for k, _ := c.Seek(start); k != nil && (end == nil || bytes.Compare(k, end) < 0); k, _ = c.Next() {
				idx++
				if idx < offset-1 {
					continue
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}

				if !yield(Key(cp(k)[:]), nil) {
					return nil
				}

				if limit > 0 && idx >= offset+limit-1 {
					return nil
				}
			}

			return nil
		})

		if err != nil {
			yield(emptyKey, fmt.Errorf("btkv range keys: %w", err))
		}
	}
}

// RangeValues returns an iterator over the entities in the store between the given time range.
func (r *BoltTKV) RangeValues(
	ctx context.Context,
	from, to *time.Time,
	offset, limit int,
) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		var idx int
		err := r.db.View(func(tx *bolt.Tx) error {
			c := r.bucket(tx).Cursor()

			var start, end []byte
			if from != nil {
				start = MarshalTime(*from, emptyKey).Bytes()
			}

			if to != nil {
				end = MarshalTime(*to, emptyKey).Bytes()
			}

			for k, v := c.Seek(start); k != nil && (end == nil || bytes.Compare(k, end) < 0); k, v = c.Next() {
				idx++
				if idx < offset {
					continue
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}

				if !yield(cp(v), nil) {
					return nil
				}

				if limit > 0 && idx >= offset+limit {
					return nil
				}
			}

			return nil
		})

		if err != nil {
			yield(nil, fmt.Errorf("btkv range values: %w", err))
		}
	}
}

func (r *BoltTKV) buckets(tx *bolt.Tx) (bkt *bolt.Bucket, idxBkt *bolt.Bucket) {
	return r.bucket(tx), r.index(tx)
}

func (r *BoltTKV) bucket(tx *bolt.Tx) *bolt.Bucket {
	return tx.Bucket(r.mainBucket)
}

func (r *BoltTKV) index(tx *bolt.Tx) *bolt.Bucket {
	return tx.Bucket(r.indexBucket)
}

func cp(b []byte) (c []byte) {
	if b == nil {
		return nil
	}

	c = make([]byte, len(b))
	copy(c, b)

	return
}

func s2b(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s)) // nolint:gosec // it's ok 👌
}
