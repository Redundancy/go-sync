package comparer

import (
	"bytes"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
	"testing"
)

func CheckResults(
	t *testing.T,
	original, modified string,
	results <-chan BlockMatchResult,
	block_size uint,
	expectedCount uint,
) {
	var resultStrings []string

	for i := range results {

		if i.Err != nil {
			t.Fatal(i.Err)
		}

		os := i.BlockIdx * block_size
		original_max := os + block_size
		if original_max > uint(len(original)) {
			original_max = uint(len(original))
		}

		orignal := original[os:original_max]

		compare_max := i.ComparisonOffset + int64(block_size)
		if compare_max > int64(len(modified)) {
			compare_max = int64(len(modified))
		}

		compare := modified[i.ComparisonOffset:compare_max]

		if orignal != compare {
			t.Errorf(
				"Bad match: \"%v\" to \"%v\" : %#v",
				orignal, compare,
				i,
			)
		}

		resultStrings = append(resultStrings, orignal)
	}

	if len(resultStrings) != int(expectedCount) {
		t.Errorf("%v blocks should have matched, only got: %v", expectedCount, resultStrings)
	}
}

func TestDetectsPrependedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error

	const ORIGINAL_STRING = "abcdefghijklmnop"
	const PREPENDED_STRING = "12" + ORIGINAL_STRING

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(PREPENDED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		PREPENDED_STRING,
		results,
		BLOCK_SIZE,
		4,
	)
}

func TestDetectsInjectedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefgh"
	const B = "ijklmnop"
	const ORIGINAL_STRING = A + B
	const MODIFIED_STRING = A + "23" + B

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(MODIFIED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		4,
	)
}

func TestDetectsAppendedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error

	const ORIGINAL_STRING = "abcdefghijklmnop"
	const MODIFIED_STRING = ORIGINAL_STRING + "23"

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(MODIFIED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		4,
	)

}

func TestDetectsModifiedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefgh"
	const B = "ijkl"
	const C = "mnop"
	const ORIGINAL_STRING = A + B + C
	const MODIFIED_STRING = A + "i2kl" + C

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(MODIFIED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		3,
	)
}

func TestDetectsPartialBlockAtEnd(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefghijklmnopqrstuvwxyz"
	const ORIGINAL_STRING = A
	const MODIFIED_STRING = A

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(MODIFIED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		7, // [abcd efgh ijkl mnop qrst uvwx yz]
	)
}

func TestMultipleResultsForDuplicatedBlocks(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcd"
	const ORIGINAL_STRING = A + A
	const MODIFIED_STRING = A

	originalFileContent := bytes.NewBufferString(ORIGINAL_STRING)
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)
	_, reference, err := indexbuilder.BuildChecksumIndex(generator, originalFileContent)

	if err != nil {
		t.Fatal(err)
	}

	modifiedContent := bytes.NewBufferString(MODIFIED_STRING)

	results := (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		2, // [abcd abcd]
	)
}
