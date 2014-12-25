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
	expectedResults []string,
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

	if len(resultStrings) != len(expectedResults) {
		t.Fatalf(
			"%v blocks should have matched, only got: %v",
			len(expectedResults),
			resultStrings,
		)
	}

	for i, v := range expectedResults {
		if resultStrings[i] != v {
			t.Errorf("%v != %v", resultStrings[i], v)
		}
	}
}

func compare(
	original string,
	modified string,
	block_size uint,
) (results <-chan BlockMatchResult, err error) {

	originalFileContent := bytes.NewBufferString(original)
	generator := filechecksum.NewFileChecksumGenerator(block_size)

	_, reference, err := indexbuilder.BuildChecksumIndex(
		generator,
		originalFileContent,
	)

	if err != nil {
		return
	}

	modifiedContent := bytes.NewBufferString(modified)

	results = (&Comparer{}).StartFindMatchingBlocks(
		modifiedContent,
		0,
		generator,
		reference,
	)

	return
}

func TestDetectsPrependedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error

	const ORIGINAL_STRING = "abcdefghijklmnop"
	const PREPENDED_STRING = "12" + ORIGINAL_STRING

	results, err := compare(ORIGINAL_STRING, PREPENDED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		PREPENDED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop"},
	)
}

func TestDetectsInjectedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefgh"
	const B = "ijklmnop"
	const ORIGINAL_STRING = A + B
	const MODIFIED_STRING = A + "23" + B

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop"},
	)
}

func TestDetectsAppendedContent(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error

	const ORIGINAL_STRING = "abcdefghijklmnop"
	const MODIFIED_STRING = ORIGINAL_STRING + "23"

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop"},
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

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", C},
	)
}

func TestDetectsPartialBlockAtEnd(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefghijklmnopqrstuvwxyz"
	const ORIGINAL_STRING = A
	const MODIFIED_STRING = A

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop", "qrst", "uvwx", "yz"},
	)
}

func TestDetectsModifiedPartialBlockAtEnd(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefghijklmnopqrstuvwx"
	const ORIGINAL_STRING = A + "yz"
	const MODIFIED_STRING = A + "23"

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}
	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop", "qrst", "uvwx"},
	)
}

func TestDetectsUnmodifiedPartialBlockAtEnd(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcdefghijklmnopqrst"
	const ORIGINAL_STRING = A + "uvwx" + "yz"
	const MODIFIED_STRING = A + "us6x" + "yz"

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{"abcd", "efgh", "ijkl", "mnop", "qrst", "yz"},
	)
}

func TestMultipleResultsForDuplicatedBlocks(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const A = "abcd"
	const ORIGINAL_STRING = A + A
	const MODIFIED_STRING = A

	results, err := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	if err != nil {
		t.Fatal(err)
	}

	CheckResults(
		t,
		ORIGINAL_STRING,
		MODIFIED_STRING,
		results,
		BLOCK_SIZE,
		[]string{A, A},
	)
}
