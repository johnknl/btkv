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
	"encoding/binary"
	"strconv"
	"time"
)

// Key is a 16 byte array, representing a key in the key-value store.
//
// The layout is 8 bytes for the timestamp, 4 bytes for a sequence counter,
// and 4 bytes reserved for future use.
//
// The timestamp is stored as an 64-bit number representing the number of nanoseconds.
//
// It differs in this way from time.Time.MarshalBinary, which uses:
// - 1 byte for the version (0x01)
// - 8 bytes for the seconds since epoch (int64)
// - 4 bytes for the nanoseconds (int32)
// - 2/3 bytes for the timezone
//
// A btkv Key is also 16 bytes, but only half of it is used for the timestamp.
//
// This does mean that the timestamp is limited to representing timestamps between
// 1677-09-21 and 2262-04-11.
type Key [16]byte

// Bytes returns the byte slice representation of the Key.
func (k Key) Bytes() []byte {
	return k[:]
}

// SplitMix64 returns a 64-bit pseudo-hash of the Key, using a mixing function to ensure
// a good distribution of bits (aka _SplitMix64_).
func (k Key) SplitMix64() uint64 {
	x := binary.BigEndian.Uint64(k[:8]) ^
		uint64(binary.BigEndian.Uint32(k[8:]))

	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31

	return x
}

// UnmarshalTime takes a Key and returns a time.Time value.
func (k Key) UnmarshalTime() time.Time {
	u := binary.BigEndian.Uint64(k[0:8]) ^ (1 << 63)
	n := int64(u) // nolint:gosec // safe

	return time.Unix(0, n)
}

// String returns a string representation of the Key, which includes the timestamp and sequence counter.
func (k Key) String() string {
	return k.UnmarshalTime().Format(time.RFC3339Nano) +
		"-" + strconv.Itoa(int(binary.BigEndian.Uint32(k[8:12]))) // nolint:gosec // overflow smoverflow
}

// NewKey creates a new Key from a byte slice.
func NewKey(b []byte) Key {
	var k Key
	copy(k[:], b)
	return k
}

// MarshalTime takes a time.Time value and returns a Key
//
// The timestamp is stored as a 64-bit integer (8 bytes) in big-endian order, followed
// by a 64-bit sequence counter (8 bytes) in big-endian order.
// When a previous Key is provided, the sequence counter is incremented to ensure
// uniqueness for keys with the same timestamp.
// If no previous Key is provided, the sequence counter is set to 0.
func MarshalTime(t time.Time, prev Key) Key {
	// Flip the sign bit to ensure that negative timestamps are sorted before positive timestamps
	n := uint64(t.UnixNano()) ^ (1 << 63)

	var seq uint32

	var key Key
	binary.BigEndian.PutUint64(key[0:8], n) // put timestamp bytes

	if bytes.Equal(prev[0:8], key[0:8]) {
		seq = binary.BigEndian.Uint32(prev[8:12])
		seq++
	}

	binary.BigEndian.PutUint32(key[8:12], seq)

	return key
}
