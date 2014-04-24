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
		orignal := original[os : os+block_size]
		compare := modified[i.ComparisonOffset : i.ComparisonOffset+int64(block_size)]

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

	results := FindMatchingBlocks(
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

	results := FindMatchingBlocks(
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

	results := FindMatchingBlocks(
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

	results := FindMatchingBlocks(
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
