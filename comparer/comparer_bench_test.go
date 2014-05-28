package comparer

import (
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/util/readers"
	"testing"
)

var test = []byte{0, 1, 2, 3}

type NegativeWeakIndex struct {
}

func (i *NegativeWeakIndex) FindWeakChecksum2(chk []byte) interface{} {
	return nil
}

func (i *NegativeWeakIndex) FindStrongChecksum2(chk []byte, weak interface{}) []chunks.ChunkChecksum {
	return nil
}

type NegativeStrongIndex struct {
}

func (i *NegativeStrongIndex) FindWeakChecksum2(chk []byte) interface{} {
	return i
}

func (i *NegativeStrongIndex) FindStrongChecksum2(chk []byte, weak interface{}) []chunks.ChunkChecksum {
	return nil
}

func BenchmarkWeakComparison(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)

	const BLOCK_SIZE = 8
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)

	b.StartTimer()

	results := (&Comparer{}).StartFindMatchingBlocks(
		readers.OneReader(b.N+BLOCK_SIZE),
		0,
		generator,
		&NegativeWeakIndex{},
	)

	for _, ok := <-results; ok; {
	}

	b.StopTimer()
}

func BenchmarkStrongComparison(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)

	const BLOCK_SIZE = 8
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)

	b.StartTimer()

	results := (&Comparer{}).StartFindMatchingBlocks(
		readers.OneReader(b.N+BLOCK_SIZE),
		0,
		generator,
		&NegativeStrongIndex{},
	)

	for _, ok := <-results; ok; {
	}

	b.StopTimer()
}
