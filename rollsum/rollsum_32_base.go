package rollsum

import (
	"encoding/binary"
)

const FULL_BYTES_16 = (1 << 16) - 1

// Rollsum32Base decouples the rollsum algorithm from the implementation of
// hash.Hash and the storage the rolling checksum window
// this allows us to write different versions of the storage for the distinctly different
// use-cases and optimize the storage with the usage pattern.
func NewRollsum32Base(blockSize uint) *Rollsum32Base {
	return &Rollsum32Base{blockSize: blockSize}
}

// The specification of hash.Hash is such that it cannot be implemented without implementing storage
// but the most optimal storage scheme depends on usage of the circular buffer & hash
type Rollsum32Base struct {
	blockSize uint
	a, b      uint32
}

// Add a single byte into the rollsum
func (r *Rollsum32Base) AddByte(b byte) {
	r.a += uint32(b)
	r.b += r.a
}

func (r *Rollsum32Base) AddBytes(bs []byte) {
	for _, b := range bs {
		r.a += uint32(b)
		r.b += r.a
	}
}

// Remove a byte from the end of the rollsum
// Use the previous length (before removal)
func (r *Rollsum32Base) RemoveByte(b byte, length int) {
	r.a -= uint32(b)
	r.b -= uint32(uint(length) * uint(b))
}

func (r *Rollsum32Base) RemoveBytes(bs []byte, length int) {
	for _, b := range bs {
		r.a -= uint32(b)
		r.b -= uint32(uint(length) * uint(b))
		length -= 1
	}
}

func (r *Rollsum32Base) AddAndRemoveBytes(add []byte, remove []byte, length int) {
	len_added := len(add)
	len_removed := len(remove)

	startEvicted := len_added - len_removed
	r.AddBytes(add[:startEvicted])
	length += startEvicted

	for i := startEvicted; i < len_added; i++ {
		r.RemoveByte(remove[i-startEvicted], length)
		r.AddByte(add[i])
	}
}

// Set a whole block of blockSize
func (r *Rollsum32Base) SetBlock(block []byte) {
	r.Reset()
	r.AddBytes(block)
}

// Reset the hash to the initial state
func (r *Rollsum32Base) Reset() {
	r.a, r.b = 0, 0
}

// size of the hash in bytes
func (r *Rollsum32Base) Size() int {
	return 4
}

// Puts the sum into b. Avoids allocation. b must have length >= 4
func (r *Rollsum32Base) GetSum(b []byte) {
	value := uint32((r.a & FULL_BYTES_16) + ((r.b & FULL_BYTES_16) << 16))
	binary.LittleEndian.PutUint32(b, value)
}
