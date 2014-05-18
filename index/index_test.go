package index

import (
	"github.com/Redundancy/go-sync/chunks"
	"testing"
)

func TestMakeIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{0, []byte("a"), []byte("b")},
			{1, []byte("b"), []byte("c")},
		},
	)

	if len(i.weakChecksumLookup) != 2 {
		t.Errorf("size of lookup was not expected %v", len(i.weakChecksumLookup))
	}
}

func TestFindWeakInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{0, []byte("a"), []byte("b")},
			{1, []byte("b"), []byte("c")},
			{2, []byte("b"), []byte("d")},
		},
	)

	result := i.FindWeakChecksumInIndex([]byte("b"))

	if result == nil {
		t.Error("Did not find lookfor in the index")
	} else if len(result) != 2 {
		t.Errorf("Wrong number of possible matches found: %v", len(result))
	} else if result[0].ChunkOffset != 1 {
		t.Errorf("Found chunk had offset %v expected 1", result[0].ChunkOffset)
	}
}

func TestFindStrongInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{0, []byte("a"), []byte("b")},
			{1, []byte("b"), []byte("c")},
			{2, []byte("b"), []byte("d")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex([]byte("b"))
	strongs := result.FindStrongChecksum([]byte("c"))

	if len(strongs) != 1 {
		t.Errorf("Incorrect number of strong checksums found: %v", len(strongs))
	} else if strongs[0].ChunkOffset != 1 {
		t.Errorf("Wrong chunk found, had offset %v", strongs[0].ChunkOffset)
	}
}

func TestFindDuplicatedBlocksInIndex(t *testing.T) {
	i := MakeChecksumIndex(
		[]chunks.ChunkChecksum{
			{0, []byte("a"), []byte("b")},
			{1, []byte("b"), []byte("c")},
			{3, []byte("b"), []byte("c")},
			{2, []byte("b"), []byte("d")},
		},
	)

	// builds upon TestFindWeakInIndex
	result := i.FindWeakChecksumInIndex([]byte("b"))
	strongs := result.FindStrongChecksum([]byte("c"))

	if len(strongs) != 2 {
		t.Fatalf("Incorrect number of strong checksums found: %v", strongs)
	}

	if strongs[0].ChunkOffset != 1 {
		t.Errorf("Wrong chunk found, had offset %v", strongs[0].ChunkOffset)
	}
	if strongs[1].ChunkOffset != 3 {
		t.Errorf("Wrong chunk found, had offset %v", strongs[1].ChunkOffset)
	}
}
