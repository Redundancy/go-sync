package index

import (
	"testing"

	"github.com/Redundancy/go-sync/chunks"
)

// Weak checksums must be 4 bytes
var WEAK_A = []byte("aaaa")
var WEAK_B = []byte("bbbb")

/*
ChunkOffset uint
// the size of the block
Size           int64
WeakChecksum   []byte
StrongChecksum []byte
*/

func TestMakeIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
		},
	)

	if i.Count != 2 {
		t.Fatalf("Wrong count on index %v", i.Count)
	}
}

func TestFindWeakInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	result := i.FindWeakChecksumInIndex(WEAK_B)

	if result == nil {
		t.Error("Did not find lookfor in the index")
	} else if len(result) != 2 {
		t.Errorf("Wrong number of possible matches found: %v", len(result))
	} else if result[0].ChunkOffset != 1 {
		t.Errorf("Found chunk had offset %v expected 1", result[0].ChunkOffset)
	}
}

func TestWeakNotInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	result := i.FindWeakChecksumInIndex([]byte("afgh"))

	if result != nil {
		t.Error("Result from FindWeakChecksumInIndex should be nil")
	}

	result2 := i.FindWeakChecksum2([]byte("afgh"))

	if result2 != nil {
		t.Errorf("Result from FindWeakChecksum2 should be nil: %#v", result2)
	}
}

func TestWeakNotInIndex2(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	result := i.FindWeakChecksumInIndex([]byte("llll"))

	if result != nil {
		t.Error("Result should be nil")
	}
}

func TestFindStrongInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex(WEAK_B)
	strongs := result.FindStrongChecksum([]byte("c"))

	if len(strongs) != 1 {
		t.Errorf("Incorrect number of strong checksums found: %v", len(strongs))
	} else if strongs[0].ChunkOffset != 1 {
		t.Errorf("Wrong chunk found, had offset %v", strongs[0].ChunkOffset)
	}
}

func TestNotFoundStrongInIndexAtEnd(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex(WEAK_B)
	strongs := result.FindStrongChecksum([]byte("e"))

	if len(strongs) != 0 {
		t.Errorf("Incorrect number of strong checksums found: %v", strongs)
	}
}

func TestNotFoundStrongInIndexInCenter(t *testing.T) {
	// The strong checksum we're looking for is not found
	// but is < another checksum in the strong list

	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
			{ChunkOffset: 3, WeakChecksum: WEAK_B, StrongChecksum: []byte("f")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex(WEAK_B)
	strongs := result.FindStrongChecksum([]byte("e"))

	if len(strongs) != 0 {
		t.Errorf("Incorrect number of strong checksums found: %v", strongs)
	}
}

func TestFindDuplicatedBlocksInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 0, WeakChecksum: WEAK_A, StrongChecksum: []byte("b")},
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 3, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("d")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex(WEAK_B)
	strongs := result.FindStrongChecksum([]byte("c"))

	if len(strongs) != 2 {
		t.Fatalf("Incorrect number of strong checksums found: %v", strongs)
	}

	first := strongs[0]
	if first.ChunkOffset != 1 {
		t.Errorf("Wrong chunk found, had offset %v", first.ChunkOffset)
	}

	second := strongs[1]
	if second.ChunkOffset != 3 {
		t.Errorf("Wrong chunk found, had offset %v", second.ChunkOffset)
	}
}

func TestFindTwoDuplicatedBlocksInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{ChunkOffset: 1, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
			{ChunkOffset: 2, WeakChecksum: WEAK_B, StrongChecksum: []byte("c")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex(WEAK_B)
	strongs := result.FindStrongChecksum([]byte("c"))

	if len(strongs) != 2 {
		t.Fatalf("Incorrect number of strong checksums found: %v", strongs)
	}

	first := strongs[0]
	if first.ChunkOffset != 1 {
		t.Errorf("Wrong chunk found, had offset %v", first.ChunkOffset)
	}

	second := strongs[1]
	if second.ChunkOffset != 2 {
		t.Errorf("Wrong chunk found, had offset %v", second.ChunkOffset)
	}
}
