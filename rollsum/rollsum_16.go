/*
rollsum implements the rolling checksum algorithm for weak checksums, based on the
writeup here: http://tutorials.jenkov.com/rsync/checksums.html

The implementation of Rollsum16 is not used internally, but is provided for convenience and
completeness. The internal implementations rely on Rollsum16Base, which provides only the
hashing mechanics without the storage, and allows the implementation to be optimized by usage.

TODO: Port over to the C2 circular buffer so that performance isn't totally sucky
*/
package rollsum

import (
	"github.com/Redundancy/go-sync/circularbuffer"
)

func NewRollsum16(blocksize uint) *Rollsum16 {
	return &Rollsum16{
		Rollsum16Base: Rollsum16Base{
			blockSize: blocksize,
		},
		buffer: circularbuffer.NewCircularBuffer(int64(blocksize)),
	}
}

// Uses 16bit internal values, 4 byte hashes
type Rollsum16 struct {
	Rollsum16Base
	buffer *circularbuffer.CircularBuffer
}

// cannot be called concurrently
func (r *Rollsum16) Write(p []byte) (n int, err error) {
	overwritten := r.buffer.WriteEvicted(p)
	remaining := p
	ulen_p := uint(len(p))

	if ulen_p < r.blockSize {
		r.RemoveBytes(overwritten)
	} else {
		r.Rollsum16Base.Reset()
	}

	if ulen_p > r.blockSize {
		// if it's really long, we can just ignore a load of it
		remaining = p[ulen_p-r.blockSize:]
	}

	r.AddBytes(remaining)
	return len(p), nil
}

func (r *Rollsum16) BlockSize() int {
	return int(r.blockSize)
}

func (r *Rollsum16) Size() int {
	return 4
}

func (r *Rollsum16) Reset() {
	r.Rollsum16Base.Reset()
	r.buffer = circularbuffer.NewCircularBuffer(int64(r.blockSize))
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (r *Rollsum16) Sum(b []byte) []byte {
	result := make([]byte, 4)
	r.Rollsum16Base.GetSum(result)

	if b != nil {
		return append(b, result...)
	} else {
		return result
	}
}

func (r *Rollsum16) GetLastBlock() []byte {
	return r.buffer.Get()
}
