package gosync

import (
	"bytes"
	"fmt"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
)

func Example() {
	// due to short example strings, use a very small block size
	// using one this small in practice would increase your file transfer!
	const BLOCK_SIZE = 4

	// This is the "file" as described by the authoritive version
	const REFERENCE = "The quick brown fox jumped over the lazy dog"

	// This is what we have locally. Not too far off, but not correct.
	const LOCAL_VERSION = "The qwik brown fox jumped 0v3r the lazy dog"

	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)

	_, referenceFileIndex, err := indexbuilder.BuildIndexFromString(generator, REFERENCE)

	if err != nil {
		return
	}

	// This will result in a stream of blocks that match in the local version
	// to those in the reference
	// We could do this on two goroutines simultaneously, if we used two identical generators
	matchStream := comparer.FindMatchingBlocks(
		bytes.NewBufferString(LOCAL_VERSION),
		0,
		generator,
		referenceFileIndex,
	)

	merger := &comparer.MatchMerger{}

	// Combine adjacent blocks. If finding concurrently, call once per stream
	merger.MergeResults(matchStream, BLOCK_SIZE)

	// a sorted list of ranges of blocks that match between the reference and the local version
	matchingBlockRanges := merger.GetMergedBlocks()

	for _, matchingRange := range matchingBlockRanges {
		localMatchStart := matchingRange.ComparisonStartOffset
		localMatchEnd := matchingRange.EndOffset(BLOCK_SIZE)

		fmt.Printf(
			"Match: \"%v\"\n",
			LOCAL_VERSION[localMatchStart:localMatchEnd],
		)
	}

	missingBlockRanges := matchingBlockRanges.GetMissingBlocks(uint(referenceFileIndex.BlockCount))

	for _, missingRange := range missingBlockRanges {
		referenceStart := missingRange.StartBlock * BLOCK_SIZE
		referenceEnd := (missingRange.EndBlock + 1) * BLOCK_SIZE

		if referenceEnd > uint(len(REFERENCE)) {
			referenceEnd = uint(len(REFERENCE))
		}

		fmt.Printf(
			"Missing: \"%v\"\n",
			REFERENCE[referenceStart:referenceEnd],
		)
	}

	// Output:
	// Match: "The "
	// Match: "k brown fox jump"
	// Match: "the lazy dog"
	// Missing: "quic"
	// Missing: "ed over "
}
