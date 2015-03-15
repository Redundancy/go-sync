package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/index"
	"github.com/Redundancy/go-sync/patcher"
	"io"
	"net/http"
	"net/url"
	"os"
)

const (
	KB = 1024
	MB = 1024 * KB
)

func openFileAndHandleError(filename string) (f *os.File) {
	var err error
	f, err = os.Open(filename)

	if err != nil {
		f = nil
		handleFileError(filename, err)
	}

	return
}

func formatFileError(filename string, err error) error {
	switch {
	case os.IsExist(err):
		return fmt.Errorf(
			"Could not open %v (already exists): %v",
			filename,
			err,
		)
	case os.IsNotExist(err):
		return fmt.Errorf(
			"Could not find %v: %v\n",
			filename,
			err,
		)
	case os.IsPermission(err):
		return fmt.Errorf(
			"Could not open %v (permission denied): %v\n",
			filename,
			err,
		)
	default:
		return fmt.Errorf(
			"Unknown error opening %v: %v\n",
			filename,
			err,
		)
	}
}

func handleFileError(filename string, err error) {
	e := formatFileError(filename, err)
	fmt.Fprintln(os.Stderr, e)
}

func getLocalOrRemoteFile(path string) (io.ReadCloser, error) {
	url, err := url.Parse(path)

	switch {
	case err != nil:
		return os.Open(path)
	case url.Scheme == "":
		return os.Open(path)
	default:
		response, err := http.Get(path)

		if err != nil {
			return nil, err
		}

		if response.StatusCode < 200 || response.StatusCode > 299 {
			return nil, fmt.Errorf("Request to %v returned status: %v", path, response.Status)
		}

		return response.Body, nil
	}
}

func toPatcherFoundSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.FoundBlockSpan {
	result := make([]patcher.FoundBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].MatchOffset = v.ComparisonStartOffset
		result[i].BlockSize = blockSize
	}

	return result
}

func toPatcherMissingSpan(sl comparer.BlockSpanList, blockSize int64) []patcher.MissingBlockSpan {
	result := make([]patcher.MissingBlockSpan, len(sl))

	for i, v := range sl {
		result[i].StartBlock = v.StartBlock
		result[i].EndBlock = v.EndBlock
		result[i].BlockSize = blockSize
	}

	return result
}

func write_headers(f *os.File, magic string, blocksize uint32, filesize int64, versions []uint16) (err error) {
	if _, err = f.WriteString(magic_string); err != nil {
		return
	}

	for _, v := range versions {
		if err = binary.Write(f, binary.LittleEndian, v); err != nil {
			return
		}
	}

	if err = binary.Write(f, binary.LittleEndian, filesize); err != nil {
		return
	}

	err = binary.Write(f, binary.LittleEndian, blocksize)
	return
}

// reads the file headers and checks the magic string, then the semantic versioning
func read_headers_and_check(r io.Reader, magic string, required_major_version uint16) (major, minor, patch uint16, filesize int64, blocksize uint32, err error) {
	b := make([]byte, len(magic_string))

	if _, err = r.Read(b); err != nil {
		return
	} else if string(b) != magic_string {
		err = errors.New("file header does not match magic string. Not a valid gosync file.")
		return
	}

	for _, v := range []*uint16{&major, &minor, &patch} {
		err = binary.Read(r, binary.LittleEndian, v)
		if err != nil {
			return
		}
	}

	if required_major_version != major {
		err = fmt.Errorf(
			"The major version of the gosync file (%v.%v.%v) does not match the tool (%v.%v.%v).",
			major, minor, patch,
			major_version, minor_version, patch_version,
		)

		return
	}

	err = binary.Read(r, binary.LittleEndian, &filesize)
	if err != nil {
		return
	}

	err = binary.Read(r, binary.LittleEndian, &blocksize)
	return
}

func read_index(r io.Reader, blocksize uint) (i *index.ChecksumIndex, err error) {
	generator := filechecksum.NewFileChecksumGenerator(blocksize)

	readChunks, e := chunks.LoadChecksumsFromReader(
		r,
		generator.WeakRollingHash.Size(),
		generator.StrongHash.Size(),
	)

	err = e

	if err != nil {
		return
	}

	i = index.MakeChecksumIndex(readChunks)

	return
}

func multithreaded_matching(
	local_file *os.File,
	idx *index.ChecksumIndex,
	local_file_size,
	num_matchers int64,
	blocksize uint,
) (*comparer.MatchMerger, *comparer.Comparer) {
	// Note: Since not all sections of the file are equal in work
	// it would be better to divide things up into more sections and
	// pull work from a queue channel as each finish
	sectionSize := local_file_size / num_matchers
	sectionSize += int64(blocksize) - (sectionSize % int64(blocksize))
	merger := &comparer.MatchMerger{}
	compare := &comparer.Comparer{}

	for i := int64(0); i < num_matchers; i++ {
		offset := sectionSize * i

		// Sections must overlap by blocksize (strictly blocksize - 1?)
		if i > 0 {
			offset -= int64(blocksize)
		}

		sectionReader := bufio.NewReaderSize(
			io.NewSectionReader(local_file, offset, sectionSize),
			MB,
		)

		sectionGenerator := filechecksum.NewFileChecksumGenerator(uint(blocksize))

		matchStream := compare.StartFindMatchingBlocks(
			sectionReader, offset, sectionGenerator, idx)

		merger.StartMergeResultStream(matchStream, int64(blocksize))
	}

	return merger, compare
}

// better way to do this?
func is_same_file(path1, path2 string) (same bool, err error) {

	fi1, err := os.Stat(path1)

	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return
	}

	fi2, err := os.Stat(path2)

	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return
	}

	return os.SameFile(fi1, fi2), nil
}
