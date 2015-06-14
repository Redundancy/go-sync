package comparer

import (
	"bufio"
	"bytes"
	"io"
	"reflect"
	"testing"

	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/indexbuilder"
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
			"%#v blocks should have matched, got: %#v",
			len(expectedResults),
			resultStrings,
		)
	}

	for i, v := range expectedResults {
		if resultStrings[i] != v {
			t.Errorf("%#v != %#v", resultStrings[i], v)
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

	_, reference, _, err := indexbuilder.BuildChecksumIndex(
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

// Splits successive strings into blocks of size n
// 2, ABCD -> AB, CD
// 2, ABCD, E, FG -> AB, CD, E, FG
func split(n int, ss ...string) (result []string) {
	totalLength := 0
	for _, s := range ss {
		totalLength += len(s)/n + 1
	}
	result = make([]string, 0, totalLength)

	for _, x := range ss {
		i := int(0)
		for i+n < len(x) {
			result = append(
				result,
				x[i:i+n],
			)

			i += n
		}

		if i < len(x)-1 {
			result = append(
				result,
				x[i:],
			)
		}
	}

	return
}

func TestSplit(t *testing.T) {
	INPUT := "abcdef"
	EXPECTED := []string{"ab", "cd", "ef"}
	result := split(2, INPUT)

	if !reflect.DeepEqual(result, EXPECTED) {
		t.Errorf(
			"Lists differ: %v vs %v",
			result,
			EXPECTED,
		)
	}
}

func TestSplitWithPartial(t *testing.T) {
	INPUT := "abcdef"
	EXPECTED := []string{"abcd", "ef"}
	result := split(4, INPUT)

	if !reflect.DeepEqual(result, EXPECTED) {
		t.Errorf(
			"Lists differ: %v vs %v",
			result,
			EXPECTED,
		)
	}
}

func TestMultiSplit(t *testing.T) {
	INPUT := []string{"abcdef", "ghij"}
	EXPECTED := []string{"abcd", "ef", "ghij"}
	result := split(4, INPUT...)

	if !reflect.DeepEqual(result, EXPECTED) {
		t.Errorf(
			"Lists differ: %v vs %v",
			result,
			EXPECTED,
		)
	}
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
		split(4, ORIGINAL_STRING),
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
		split(4, A, B),
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
		split(4, ORIGINAL_STRING),
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
		split(4, A, C),
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
		split(4, A),
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
		split(4, A),
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
		split(4, A, "yz"),
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

func TestRegression1(t *testing.T) {
	const BLOCK_SIZE = 4
	var err error
	const ORIGINAL_STRING = "The quick brown fox jumped over the lazy dog"
	const MODIFIED_STRING = "The qwik brown fox jumped 0v3r the lazy"

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
		[]string{"The ", "k br", "own ", "fox ", "jump", "the ", "lazy"},
	)
}

func TestTwoComparisons(t *testing.T) {
	const BLOCK_SIZE = 4
	const ORIGINAL_STRING = "The quick brown fox jumped over the lazy dog"
	const MODIFIED_STRING = "The qwik brown fox jumped 0v3r the lazy"

	numMatchers := int64(4)
	sectionSize := int64(len(ORIGINAL_STRING)) / numMatchers
	sectionSize += int64(BLOCK_SIZE) - (sectionSize % int64(BLOCK_SIZE))

	merger := &MatchMerger{}

	originalFile := bytes.NewReader([]byte(ORIGINAL_STRING))
	modifiedFile := bytes.NewReader([]byte(MODIFIED_STRING))
	generator := filechecksum.NewFileChecksumGenerator(BLOCK_SIZE)

	_, reference, _, _ := indexbuilder.BuildChecksumIndex(
		generator,
		originalFile,
	)

	for i := int64(0); i < numMatchers; i++ {
		compare := &Comparer{}
		offset := sectionSize * i

		t.Logf("Section %v: %v-%v", i, offset, offset+sectionSize)

		sectionReader := bufio.NewReaderSize(
			io.NewSectionReader(modifiedFile, offset, sectionSize+BLOCK_SIZE),
			100000, // 1 MB buffer
		)

		// Bakes in the assumption about how to generate checksums (extract)
		sectionGenerator := filechecksum.NewFileChecksumGenerator(
			uint(BLOCK_SIZE),
		)

		matchStream := compare.StartFindMatchingBlocks(
			sectionReader, offset, sectionGenerator, reference,
		)

		merger.StartMergeResultStream(matchStream, int64(BLOCK_SIZE))
	}

	merged := merger.GetMergedBlocks()
	missing := merged.GetMissingBlocks(uint(len(ORIGINAL_STRING) / BLOCK_SIZE))

	expected := []string{
		"quic", "ed over ", " dog",
	}

	t.Logf("Missing blocks: %v", len(missing))

	for x, v := range missing {
		start := v.StartBlock * BLOCK_SIZE
		end := (v.EndBlock + 1) * BLOCK_SIZE
		if end > uint(len(ORIGINAL_STRING)) {
			end = uint(len(ORIGINAL_STRING))
		}
		s := ORIGINAL_STRING[start:end]

		if s != expected[x] {
			t.Errorf(
				"Wrong block %v (%v-%v): %#v (expected %#v)",
				x, v.StartBlock, v.EndBlock, s, expected[x],
			)
		} else {
			t.Logf(
				"Correct block %v (%v-%v): %#v (expected %#v)",
				x, v.StartBlock, v.EndBlock, s, expected[x],
			)
		}
	}
}
