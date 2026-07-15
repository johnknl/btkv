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
	"encoding/binary"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKey_MarshalTime_UnmarshalTime(t *testing.T) {
	now := time.Now()
	var prev Key

	t.Run("no previous key", func(t *testing.T) {
		key := MarshalTime(now, prev)
		unmarshaledTime := key.UnmarshalTime()
		require.Equal(t, now.UnixNano(), unmarshaledTime.UnixNano())
		prev = key
	})

	t.Run("previous key", func(t *testing.T) {
		key := MarshalTime(now, prev)
		unmarshaledTime := key.UnmarshalTime()
		require.Equal(t, now.UnixNano(), unmarshaledTime.UnixNano())
	})
}

func TestKey_String(t *testing.T) {
	now := time.Now()
	var prev Key

	t.Run("no previous key", func(t *testing.T) {
		key := MarshalTime(now, prev)
		keyStr := key.String()
		require.Equal(t, now.Format(time.RFC3339Nano)+"-0", keyStr)
		prev = key
	})

	t.Run("previous key", func(t *testing.T) {
		key := MarshalTime(now, prev)
		keyStr := key.String()
		require.Equal(t, now.Format(time.RFC3339Nano)+"-1", keyStr)
		prev = key

		key = MarshalTime(now, prev)
		keyStr = key.String()
		require.Equal(t, now.Format(time.RFC3339Nano)+"-2", keyStr)
	})
}

func TestKey_SplitMix64(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 2, 3, 4, time.UTC)

	key1 := Key{}
	binary.BigEndian.PutUint64(key1[:8], uint64(now.UnixNano()))
	binary.BigEndian.PutUint64(key1[8:], 42)

	key2 := Key{}
	binary.BigEndian.PutUint64(key2[:8], uint64(now.UnixNano()))
	binary.BigEndian.PutUint64(key2[8:], 43)

	key3 := Key{}
	binary.BigEndian.PutUint64(key3[:8], uint64(now.UnixNano()))
	binary.BigEndian.PutUint64(key3[8:], 44)

	t.Run("should return the same value for the same key", func(t *testing.T) {
		mix := key1.SplitMix64()
		actual := key1.SplitMix64()
		require.Equal(t, mix, actual, "Mix64 should return the same value for the same key")
	})

	t.Run("should return different values for different keys", func(t *testing.T) {
		mix1 := key1.SplitMix64()
		mix2 := key2.SplitMix64()
		require.NotEqual(t, mix1, mix2, "Mix64 should return different values for different keys")
	})

	t.Run("should return non-sequential values for sequential keys", func(t *testing.T) {
		mix1 := key1.SplitMix64()
		mix2 := key2.SplitMix64()
		mix3 := key3.SplitMix64()

		actual := []uint64{mix1, mix2, mix3}
		slices.Sort(actual)

		expected := []uint64{mix2, mix1, mix3}

		require.Equal(t, expected, actual, "Mix64 should return non-sequential values for sequential keys")
	})
}
