package gosync

import (
	"fmt"
	"testing"

	"bytes"

	"github.com/Redundancy/go-sync/blocksources"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
	"github.com/Redundancy/go-sync/util/readers"
)

func Example() {
	// due to short example strings, use a very small block size
	// using one this small in practice would increase your file transfer!
	const blockSize = 4

	// This is the "file" as described by the authoritive version
	const reference = "The quick brown fox jumped over the lazy dog"

	// This is what we have locally. Not too far off, but not correct.
	const localVersion = "The qwik brown fox jumped 0v3r the lazy"

	generator := filechecksum.NewFileChecksumGenerator(blockSize)
	_, referenceFileIndex, _, err := indexbuilder.BuildIndexFromString(
		generator,
		reference,
	)

	if err != nil {
		return
	}

	referenceAsBytes := []byte(reference)
	localVersionAsBytes := []byte(localVersion)

	blockCount := len(referenceAsBytes) / blockSize
	if len(referenceAsBytes)%blockSize != 0 {
		blockCount++
	}

	inputFile := bytes.NewReader(localVersionAsBytes)
	patchedFile := bytes.NewBuffer(nil)

	// This is more complicated than usual, because we're using in-memory
	// "files" and sources. Normally you would use MakeRSync
	summary := &BasicSummary{
		ChecksumIndex:  referenceFileIndex,
		ChecksumLookup: nil,
		BlockCount:     uint(blockCount),
		BlockSize:      blockSize,
		FileSize:       int64(len(referenceAsBytes)),
	}

	rsync := &RSync{
		Input:  inputFile,
		Output: patchedFile,
		Source: blocksources.NewReadSeekerBlockSource(
			bytes.NewReader(referenceAsBytes),
			blocksources.MakeNullFixedSizeResolver(uint64(blockSize)),
		),
		Summary: summary,
		OnClose: nil,
	}

	if err := rsync.Patch(); err != nil {
		fmt.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Patched result: \"%s\"\n", patchedFile.Bytes())
	// Output:
	// Patched result: "The quick brown fox jumped over the lazy dog"
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
	_, index, _, err := indexbuilder.BuildChecksumIndex(generator, file)

	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// must reinitialize the file for each comparison
		otherFile := readers.NewSizedNonRepeatingSequence(745656, SIZE)
		compare := &comparer.Comparer{}
		m := compare.StartFindMatchingBlocks(otherFile, 0, generator, index)

		for _, ok := <-m; ok; {
		}
	}

	b.StopTimer()
}
