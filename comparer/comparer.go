/*
package comparer is responsible for using a FileChecksumGenerator (filechecksum) and an index
to move through a file and compare it to the index, producing a FileDiffSummary
*/
package comparer

import (
	"fmt"
	"github.com/Redundancy/go-sync/circularbuffer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/index"
	"io"
)

const (
	READ_NEXT_BYTE = iota
	READ_NEXT_BLOCK
)

// If the weak Hash object satisfies this interface, then
// FindMatchingBlocks will not allocate a circular buffer
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

/*
Iterates though comparison looking for blocks that match ones from the index
it emits each block to be read from the returned channel. Callers should check for
.Err != nil on the results, in which case reading will end immediately.

FindMatchingBlocks is capable of running asyncronously
on sub-sections of a larger file. When doing this, you must overlap
by the block size, and use seperate checksum generators.
*/
func FindMatchingBlocks(
	comparison io.Reader,
	baseOffset int64,
	generator *filechecksum.FileChecksumGenerator,
	reference *index.ChecksumIndex,
) <-chan BlockMatchResult {

	resultStream := make(chan BlockMatchResult)

	go findMatchingBlocks_int(
		resultStream,
		comparison,
		baseOffset,
		generator,
		reference,
	)

	return resultStream
}

/*
type weakUpdater struct {
	hash
	weaksum []byte
}

type strongUpdater struct {
	hash hash.Hash
}
*/

/*
TODO: Refactor Weak / Strong updates / Reading + counting
*/
func findMatchingBlocks_int(
	results chan<- BlockMatchResult,
	comparison io.Reader,
	baseOffset int64,
	generator *filechecksum.FileChecksumGenerator,
	reference *index.ChecksumIndex,
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

	blockMemory := circularbuffer.NewCircularBuffer(int64(generator.BlockSize))
	blockMemory.Write(block)

	strong := generator.GetStrongHash()
	// All the bytes
	i := int64(0)
	next := READ_NEXT_BYTE

	//ReadLoop:
	for err == nil {
		// look for a weak match
		generator.WeakRollingHash.GetSum(weaksum)
		if weakMatchList := reference.FindWeakChecksumInIndex(weaksum); weakMatchList != nil {

			block = blockMemory.GetLastBlock()

			strong.Reset()
			strong.Write(block)
			strongSum = strong.Sum(strongSum)
			strongList := weakMatchList.FindStrongChecksum(strongSum)
			strongSum = strongSum[:0]

			// if there was a strong match, we only care about 1 of them
			// the assumption is that we have repeated blocks of the same data
			if len(strongList) != 0 {
				results <- BlockMatchResult{
					ComparisonOffset: i + baseOffset,
					BlockIdx:         strongList[0].ChunkOffset,
				}

				// No point looking for a match that overlaps this block
				next = READ_NEXT_BLOCK
			}
		}

		switch next {
		case READ_NEXT_BYTE:
			_, err = comparison.Read(singleByte)
			generator.WeakRollingHash.AddBytes(singleByte)
			out := blockMemory.WriteEvicted(singleByte)
			generator.WeakRollingHash.RemoveBytes(out)
			i += 1
		case READ_NEXT_BLOCK:
			_, err = io.ReadFull(comparison, block)

			if err == nil {
				generator.WeakRollingHash.SetBlock(block)
				blockMemory.WriteEvicted(block)
				i += int64(generator.BlockSize)
			} else if err == io.EOF || err == io.ErrUnexpectedEOF {
				err = io.EOF
				break
			}
			// Reset to reading bytes
			next = READ_NEXT_BYTE
		}

	}

	if err != io.EOF {
		ReportErr(err)
		return
	}
}
