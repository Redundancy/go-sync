package rollsum

import (
	"encoding/binary"
)

// 16 bits full of 1s
const FULL_BYTES_16 = (2 ^ 16) - 1

// Rollsum16Base decouples the rollsum algorithm from the implementation of
// hash.Hash and the storage the rolling checksum window
// this allows us to write different versions of the storage for the distinctly different
// use-cases and optimize the storage with the usage pattern.

func NewRollsum16Base(blockSize uint) *Rollsum16Base {
	return &Rollsum16Base{blockSize: blockSize}
}

// The specification of hash.Hash is such that it cannot be implemented without implementing storage
// but the most optimal storage scheme depends on usage of the circular buffer & hash
type Rollsum16Base struct {
	blockSize uint
	a, b      uint16
}

// Add a single byte into the rollsum
func (r *Rollsum16Base) AddByte(b byte) {
	r.a += uint16(b)
	r.b += r.a
}

func (r *Rollsum16Base) AddBytes(bs []byte) {
	for _, b := range bs {
		r.a += uint16(b)
		r.b += r.a
	}
}

// Remove a byte from the end of the rollsum
func (r *Rollsum16Base) RemoveByte(b byte) {
	r.a -= uint16(b)
	r.b -= uint16(r.blockSize * uint(b))
}

func (r *Rollsum16Base) RemoveBytes(bs []byte) {
	for _, b := range bs {
		r.a -= uint16(b)
		r.b -= uint16(r.blockSize * uint(b))
	}
}

// Set a whole block of blockSize
func (r *Rollsum16Base) SetBlock(block []byte) {
	r.Reset()
	r.AddBytes(block)
}

// Reset the hash to the initial state
func (r *Rollsum16Base) Reset() {
	r.a, r.b = 0, 0
}

// size of the hash in bytes
func (r *Rollsum16Base) Size() int {
	return 4
}

// Puts the sum into b. Avoids allocation. b must have length >= 4
func (r *Rollsum16Base) GetSum(b []byte) {
	value := uint32((r.a & FULL_BYTES_16) + ((r.b & FULL_BYTES_16) >> 16))
	binary.LittleEndian.PutUint32(b, value)
}
