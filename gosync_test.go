package gosync

import (
	"bytes"
	"fmt"
	"github.com/Redundancy/go-sync/blocksources"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
	"github.com/Redundancy/go-sync/patcher"
	"github.com/Redundancy/go-sync/patcher/sequential"
	"github.com/Redundancy/go-sync/util/readers"
	"testing"
)

func ToPatcherFoundSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.FoundBlockSpan {
	result := make([]patcher.FoundBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].MatchOffset = v.ComparisonStartOffset
		result[i].BlockSize = blockSize
	}

	return result
}

func ToPatcherMissingSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.MissingBlockSpan {
	result := make([]patcher.MissingBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].BlockSize = blockSize
	}

	return result
}

func PrintReferenceSpans(prefix string, list comparer.BlockSpanList, reference string, blockSize uint) {

	for _, missingRange := range list {
		referenceStart := missingRange.StartBlock * blockSize
		referenceEnd := (missingRange.EndBlock + 1) * blockSize

		if referenceEnd > uint(len(reference)) {
			referenceEnd = uint(len(reference))
		}

		fmt.Printf(
			"%v: \"%v\"\n",
			prefix,
			reference[referenceStart:referenceEnd],
		)
	}
}

func PrintLocalSpans(prefix string, list comparer.BlockSpanList, local string, blockSize int64) {
	for _, matchingRange := range list {
		localMatchStart := matchingRange.ComparisonStartOffset
		localMatchEnd := matchingRange.EndOffset(blockSize)

		fmt.Printf(
			"%v: \"%v\"\n",
			prefix,
			local[localMatchStart:localMatchEnd],
		)
	}
}

func Example() {
	// due to short example strings, use a very small block size
	// using one this small in practice would increase your file transfer!
	const BLOCK_SIZE = 4

	// This is the "file" as described by the authoritive version
	const REFERENCE = "The quick brown fox jumped over the lazy dog"

	// This is what we have locally. Not too far off, but not correct.
	const LOCAL_VERSION = "The qwik brown fox jumped 0v3r the lazy"

	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)

	_, referenceFileIndex, err := indexbuilder.BuildIndexFromString(generator, REFERENCE)

	if err != nil {
		return
	}

	compare := &comparer.Comparer{}

	// This will result in a stream of blocks that match in the local version
	// to those in the reference
	// We could do this on two goroutines simultaneously, if we used two identical generators
	matchStream := compare.StartFindMatchingBlocks(
		bytes.NewBufferString(LOCAL_VERSION),
		0,
		generator,
		referenceFileIndex,
	)

	merger := &comparer.MatchMerger{}

	// Combine adjacent blocks. If finding concurrently, call once per stream
	merger.StartMergeResultStream(matchStream, BLOCK_SIZE)

	// a sorted list of ranges of blocks that match between the reference and the local version
	matchingBlockRanges := merger.GetMergedBlocks()
	PrintLocalSpans("Match", matchingBlockRanges, LOCAL_VERSION, BLOCK_SIZE)

	missingBlockRanges := matchingBlockRanges.GetMissingBlocks(uint(referenceFileIndex.BlockCount) - 1)
	PrintReferenceSpans("Missing", missingBlockRanges, REFERENCE, BLOCK_SIZE)

	// the "file" to write to
	patchedFile := bytes.NewBuffer(make([]byte, 0, len(REFERENCE)))
	remoteReferenceSource := blocksources.NewReadSeekerBlockSource(
		bytes.NewReader([]byte(REFERENCE)),
	)

	err = sequential.SequentialPatcher(
		bytes.NewReader([]byte(LOCAL_VERSION)),
		remoteReferenceSource,
		ToPatcherMissingSpan(missingBlockRanges, BLOCK_SIZE),
		ToPatcherFoundSpan(matchingBlockRanges, BLOCK_SIZE),
		1024,
		patchedFile,
	)

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Patched result: \"%s\"\n", patchedFile.Bytes())
	fmt.Println("Remotely requested bytes:", remoteReferenceSource.ReadBytes(), "(without the index!)")
	fmt.Println("Full file length:", len(REFERENCE), "bytes")
	// Output:
	// Match: "The "
	// Match: "k brown fox jump"
	// Match: "the lazy"
	// Missing: "quic"
	// Missing: "ed over "
	// Missing: " dog"
	// Patched result: "The quick brown fox jumped over the lazy dog"
	// Remotely requested bytes: 16 (without the index!)
	// Full file length: 44 bytes
}

const (
	BYTE = 1
	KB   = 1024 * BYTE
	MB   = 1024 * KB
)

func BenchmarkIndexComparisons(b *testing.B) {
	b.ReportAllocs()

	const SIZE = 200 * KB
	b.SetBytes(SIZE)

	file := readers.NewSizedNonRepeatingSequence(6, SIZE)
	generator := filechecksum.NewFileChecksumGenerator(8 * KB)
	_, index, err := indexbuilder.BuildChecksumIndex(generator, file)

	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// must reinitialize the file for each comparison
		other_file := readers.NewSizedNonRepeatingSequence(745656, SIZE)
		compare := &comparer.Comparer{}
		m := compare.StartFindMatchingBlocks(other_file, 0, generator, index)

		for _, ok := <-m; ok; {
		}
	}

	b.StopTimer()
}
