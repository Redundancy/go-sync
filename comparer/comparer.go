/*
package comparer is responsible for using a FileChecksumGenerator (filechecksum) and an index
to move through a file and compare it to the index, producing a FileDiffSummary
*/
package comparer

import (
	"fmt"
	"io"
	"sync/atomic"

	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/circularbuffer"
	"github.com/Redundancy/go-sync/filechecksum"
)

const (
	READ_NEXT_BYTE = iota
	READ_NEXT_BLOCK
	READ_NONE
)

// If the weak Hash object satisfies this interface, then
// StartFindMatchingBlocks will not allocate a circular buffer
type BlockBuffer interface {
	Write([]byte) (int, error)
	// the last set of bytes of the size of the circular buffer
	// oldest to newest
	GetLastBlock() []byte
}

type BlockMatchResult struct {
	// In case of error
	Err error

	// The offset the comparison + baseOffset
	ComparisonOffset int64

	// The block from the index that it matched
	BlockIdx uint
}

type Index interface {
	FindWeakChecksum2(chk []byte) interface{}
	FindStrongChecksum2(chk []byte, weak interface{}) []chunks.ChunkChecksum
}

/*
Iterates though comparison looking for blocks that match ones from the index
it emits each block to be read from the returned channel. Callers should check for
.Err != nil on the results, in which case reading will end immediately.

StartFindMatchingBlocks is capable of running asyncronously
on sub-sections of a larger file. When doing this, you must overlap
by the block size, and use seperate checksum generators.
*/

type Comparer struct {
	Comparisons    int64
	WeakHashHits   int64
	StrongHashHits int64
}

func (c *Comparer) StartFindMatchingBlocks(
	comparison io.Reader,
	baseOffset int64,
	generator *filechecksum.FileChecksumGenerator,
	referenceIndex Index,
) <-chan BlockMatchResult {

	resultStream := make(chan BlockMatchResult)

	go c.startFindMatchingBlocks_int(
		resultStream,
		comparison,
		baseOffset,
		generator,
		referenceIndex,
	)

	return resultStream
}

/*
TODO: When matching duplicated blocks, a channel of BlockMatchResult slices would be more efficient
*/
func (c *Comparer) startFindMatchingBlocks_int(
	results chan<- BlockMatchResult,
	comparison io.Reader,
	baseOffset int64,
	generator *filechecksum.FileChecksumGenerator,
	reference Index,
) {
	defer close(results)

	block := make([]byte, generator.BlockSize)
	var err error

	ReportErr := func(err error) {
		results <- BlockMatchResult{
			Err: err,
		}
	}

	_, err = io.ReadFull(comparison, block)

	if err != nil {
		ReportErr(
			fmt.Errorf("Error reading first block in comparison: %v", err),
		)
		return
	}

	generator.WeakRollingHash.SetBlock(block)
	singleByte := make([]byte, 1)
	weaksum := make([]byte, generator.WeakRollingHash.Size())
	strongSum := make([]byte, 0, generator.GetStrongHash().Size())

	blockMemory := circularbuffer.MakeC2Buffer(int(generator.BlockSize))
	blockMemory.Write(block)

	strong := generator.GetStrongHash()
	// All the bytes
	i := int64(0)
	next := READ_NEXT_BYTE

	//ReadLoop:
	for {

		atomic.AddInt64(&c.Comparisons, 1)

		// look for a weak match
		generator.WeakRollingHash.GetSum(weaksum)
		if weakMatchList := reference.FindWeakChecksum2(weaksum); weakMatchList != nil {
			atomic.AddInt64(&c.WeakHashHits, 1)

			block = blockMemory.GetBlock()

			strong.Reset()
			strong.Write(block)
			strongSum = strong.Sum(strongSum)
			strongList := reference.FindStrongChecksum2(strongSum, weakMatchList)

			// clear the slice
			strongSum = strongSum[:0]

			// If there are many matches, it means that this block is
			// duplicated in the reference.
			// since we care about finding all the blocks in the reference,
			// we must report all of them
			off := i + baseOffset
			for _, strongMatch := range strongList {
				results <- BlockMatchResult{
					ComparisonOffset: off,
					BlockIdx:         strongMatch.ChunkOffset,
				}
			}

			if len(strongList) > 0 {
				atomic.AddInt64(&c.StrongHashHits, 1)
				if next == READ_NONE {
					// found the match at the end, so exit
					break
				}
				// No point looking for a match that overlaps this block
				next = READ_NEXT_BLOCK
			}
		}

		var n int
		var readBytes []byte

		switch next {
		case READ_NEXT_BYTE:
			n, err = comparison.Read(singleByte)
			readBytes = singleByte
		case READ_NEXT_BLOCK:
			n, err = io.ReadFull(comparison, block)
			readBytes = block[:n]
			next = READ_NEXT_BYTE
		}

		if uint(n) == generator.BlockSize {
			generator.WeakRollingHash.SetBlock(block)
			blockMemory.Write(block)
			i += int64(n)
		} else if n > 0 {
			b_len := blockMemory.Len()
			blockMemory.Write(readBytes)
			generator.WeakRollingHash.AddAndRemoveBytes(
				readBytes,
				blockMemory.Evicted(),
				b_len,
			)
			i += int64(n)
		}

		if next != READ_NONE && (err == io.EOF || err == io.ErrUnexpectedEOF) {
			err = io.EOF
			next = READ_NONE
		}

		if next == READ_NONE {
			if blockMemory.Empty() {
				break
			}

			b_len := blockMemory.Len()
			removedByte := blockMemory.Truncate(1)
			generator.WeakRollingHash.RemoveBytes(removedByte, b_len)
			i += 1
		}
	}

	if err != io.EOF {
		ReportErr(err)
		return
	}
}
