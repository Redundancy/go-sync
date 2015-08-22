package filechecksum

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/index"
	"github.com/Redundancy/go-sync/util/readers"
)

func TestChecksumGenerationEndsWithFilechecksum(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20
	emptybuffer := bytes.NewBuffer(make([]byte, BLOCK_COUNT*BLOCKSIZE))

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	lastResult := ChecksumResults{}

	for lastResult = range checksum.StartChecksumGeneration(emptybuffer, 10, nil) {
	}

	if lastResult.Checksums != nil {
		t.Errorf("Last result had checksums: %#v", lastResult)
	}

	if lastResult.Filechecksum == nil {
		t.Errorf("Last result did not contain the filechecksum: %#v", lastResult)
	}
}

func TestChecksumGenerationReturnsCorrectChecksumCount(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20

	emptybuffer := bytes.NewBuffer(make([]byte, BLOCK_COUNT*BLOCKSIZE))

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	resultCount := 0

	for r := range checksum.StartChecksumGeneration(emptybuffer, 10, nil) {
		resultCount += len(r.Checksums)
	}

	if resultCount != BLOCK_COUNT {
		t.Errorf("Unexpected block count returned: %v", resultCount)
	}
}

func TestChecksumGenerationContainsHashes(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20

	emptybuffer := bytes.NewBuffer(make([]byte, BLOCK_COUNT*BLOCKSIZE))
	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	for r := range checksum.StartChecksumGeneration(emptybuffer, 10, nil) {
		for _, r2 := range r.Checksums {
			if len(r2.WeakChecksum) != checksum.WeakRollingHash.Size() {
				t.Fatalf(
					"Wrong length weak checksum: %v vs %v",
					len(r2.WeakChecksum),
					checksum.WeakRollingHash.Size(),
				)
			}

			if len(r2.StrongChecksum) != checksum.StrongHash.Size() {
				t.Fatalf(
					"Wrong length strong checksum: %v vs %v",
					len(r2.StrongChecksum),
					checksum.StrongHash.Size(),
				)
			}
		}
	}
}

func TestRollsumLength(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20

	emptybuffer := bytes.NewBuffer(make([]byte, BLOCK_COUNT*BLOCKSIZE))
	output := bytes.NewBuffer(nil)

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	// output length is expected to be 20 blocks
	expectedLength := (BLOCK_COUNT * checksum.GetStrongHash().Size()) +
		(BLOCK_COUNT * checksum.WeakRollingHash.Size())

	_, err := checksum.GenerateChecksums(emptybuffer, output)

	if err != nil {
		t.Fatal(err)
	}

	if output.Len() != expectedLength {
		t.Errorf(
			"output length (%v) did not match expected length (%v)",
			output.Len(),
			expectedLength,
		)
	}
}

func TestRollsumLengthWithPartialBlockAtEnd(t *testing.T) {
	const BLOCKSIZE = 100
	const FULL_BLOCK_COUNT = 20
	const BLOCK_COUNT = FULL_BLOCK_COUNT + 1

	emptybuffer := bytes.NewBuffer(make([]byte, FULL_BLOCK_COUNT*BLOCKSIZE+50))
	output := bytes.NewBuffer(nil)

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	// output length is expected to be 20 blocks
	expectedLength := (BLOCK_COUNT * checksum.GetStrongHash().Size()) +
		(BLOCK_COUNT * checksum.WeakRollingHash.Size())

	_, err := checksum.GenerateChecksums(emptybuffer, output)

	if err != nil {
		t.Fatal(err)
	}

	if output.Len() != expectedLength {
		t.Errorf(
			"output length (%v) did not match expected length (%v)",
			output.Len(),
			expectedLength,
		)
	}
}

// Each of the data blocks is the same, so the checksums for the blocks should be the same
func TestChecksumBlocksTheSame(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20

	checksum := NewFileChecksumGenerator(BLOCKSIZE)
	output := bytes.NewBuffer(nil)

	_, err := checksum.GenerateChecksums(
		readers.OneReader(BLOCKSIZE*BLOCK_COUNT),
		output,
	)

	if err != nil {
		t.Fatal(err)
	}

	weakSize, strongSize := checksum.GetChecksumSizes()

	if output.Len() != BLOCK_COUNT*(strongSize+weakSize) {
		t.Errorf(
			"Unexpected output length: %v, expected %v",
			output.Len(),
			BLOCK_COUNT*(strongSize+weakSize),
		)
	}

	results, err := chunks.LoadChecksumsFromReader(output, weakSize, strongSize)

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != BLOCK_COUNT {
		t.Fatalf("Results too short! %v", len(results))
	}

	first := results[0]

	for i, chk := range results {
		if chk.ChunkOffset != uint(i) {
			t.Errorf("Unexpected offset %v on chunk %v", chk.ChunkOffset, i)
		}
		if !first.Match(chk) {
			t.Fatalf("Chunks have different checksums on %v", i)
		}
	}
}

func TestPrependedBlocks(t *testing.T) {
	const BLOCKSIZE = 100
	const BLOCK_COUNT = 20
	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	file1 := io.LimitReader(
		readers.NewNonRepeatingSequence(0),
		BLOCKSIZE*BLOCK_COUNT,
	)

	file2 := io.LimitReader(
		io.MultiReader(
			readers.OneReader(BLOCKSIZE), // Off by one block
			readers.NewNonRepeatingSequence(0),
		),
		BLOCKSIZE*BLOCK_COUNT,
	)

	output1 := bytes.NewBuffer(nil)
	chksum1, _ := checksum.GenerateChecksums(file1, output1)

	output2 := bytes.NewBuffer(nil)
	chksum2, _ := checksum.GenerateChecksums(file2, output2)

	if bytes.Compare(chksum1, chksum2) == 0 {
		t.Fatal("Checksums should be different")
	}

	weaksize, strongSize := checksum.GetChecksumSizes()
	sums1, _ := chunks.LoadChecksumsFromReader(output1, weaksize, strongSize)
	sums2, _ := chunks.LoadChecksumsFromReader(output2, weaksize, strongSize)

	if len(sums1) != len(sums2) {
		t.Fatalf("Checksum lengths differ %v vs %v", len(sums1), len(sums2))
	}

	if sums1[0].Match(sums2[0]) {
		t.Error("Chunk sums1[0] should differ from sums2[0]")
	}

	for i, _ := range sums2 {
		if i == 0 {
			continue
		}

		if !sums1[i-1].Match(sums2[i]) {
			t.Errorf("Chunk sums1[%v] equal sums2[%v]", i-1, i)
		}

	}
}

// Test that partial content that ends in the middle of a weak
// hash is caught correctly
func TestInvalidReaderLength_Weak(t *testing.T) {
	const BLOCKSIZE = 100

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	file1 := io.LimitReader(
		readers.NewNonRepeatingSequence(0),
		int64(checksum.ChecksumSize())+2,
	)

	ws, ss := checksum.GetChecksumSizes()
	r, err := chunks.LoadChecksumsFromReader(file1, ws, ss)

	if r != nil || err != chunks.ErrPartialChecksum {
		t.Error("Expected partial checksum error")
	}
}

// Test that partial content that ends in the middle of a strong
// hash is caught correctly
func TestInvalidReaderLength_Strong(t *testing.T) {
	const BLOCKSIZE = 100

	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	file1 := io.LimitReader(
		readers.NewNonRepeatingSequence(0),
		int64(checksum.ChecksumSize())+int64(checksum.WeakRollingHash.Size())+2,
	)

	ws, ss := checksum.GetChecksumSizes()
	r, err := chunks.LoadChecksumsFromReader(file1, ws, ss)

	if r != nil || err != chunks.ErrPartialChecksum {
		t.Error("Expected partial checksum error")
	}
}

func ExampleFileChecksumGenerator_LoadChecksumsFromReader() {
	const BLOCKSIZE = 8096
	checksum := NewFileChecksumGenerator(BLOCKSIZE)

	// This could be any source that conforms to io.Reader
	// sections of a file, or the body of an http response
	file1, err := os.Open("fileChecksums.chk")

	if err != nil {
		return
	}

	defer file1.Close()

	ws, ss := checksum.GetChecksumSizes()
	checksums, err := chunks.LoadChecksumsFromReader(file1, ws, ss)

	if err != nil {
		return
	}

	// Make an index that we can use against our local
	// checksums
	i := index.MakeChecksumIndex(checksums)

	// example checksum from a local file
	// look for the chunk in the index
	i.FindWeakChecksumInIndex([]byte("a"))

}
