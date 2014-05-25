package comparer

import (
	"bytes"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
	"github.com/Redundancy/go-sync/util/readers"
	"testing"
)

func BenchmarkComparison(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)

	const BLOCK_SIZE = 8
	var err error

	const ORIGINAL_STRING = "abcdefghijklmnop"

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()

	results := StartFindMatchingBlocks(
		readers.NewSizedNonRepeatingSequence(0, int64(b.N+BLOCK_SIZE)),
		0,
		generator,
		reference,
	)

	for _, ok := <-results; ok; {

	}

	b.StopTimer()
}
