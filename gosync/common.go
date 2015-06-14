package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/Redundancy/go-sync/index"
	"github.com/Redundancy/go-sync/patcher"
	"github.com/codegangsta/cli"
)

const (
	// KB - One Kilobyte
	KB = 1024
	// MB - One Megabyte
	MB = 1000000
)

func errorWrapper(c *cli.Context, f func(*cli.Context) error) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintln(os.Stderr, p)
			os.Exit(1)
		}
	}()

	if err := f(c); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	return
}

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

func writeHeaders(
	f *os.File,
	magic string,
	blocksize uint32,
	filesize int64,
	versions []uint16,
) (err error) {
	if _, err = f.WriteString(magicString); err != nil {
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
func readHeadersAndCheck(
	r io.Reader,
	magic string,
	requiredMajorVersion uint16,
) (
	major, minor, patch uint16,
	filesize int64,
	blocksize uint32,
	err error,
) {
	b := make([]byte, len(magicString))

	if _, err = r.Read(b); err != nil {
		return
	} else if string(b) != magicString {
		err = errors.New(
			"file header does not match magic string. Not a valid gosync file",
		)
		return
	}

	for _, v := range []*uint16{&major, &minor, &patch} {
		err = binary.Read(r, binary.LittleEndian, v)
		if err != nil {
			return
		}
	}

	if requiredMajorVersion != major {
		err = fmt.Errorf(
			"The major version of the gosync file (%v.%v.%v) does not match the tool (%v.%v.%v).",
			major, minor, patch,
			majorVersion, minorVersion, patchVersion,
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

func readIndex(r io.Reader, blocksize uint) (
	i *index.ChecksumIndex,
	checksumLookup filechecksum.ChecksumLookup,
	blockCount uint,
	err error,
) {
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

	checksumLookup = chunks.StrongChecksumGetter(readChunks)
	i = index.MakeChecksumIndex(readChunks)
	blockCount = uint(len(readChunks))

	return
}

func multithreadedMatching(
	localFile *os.File,
	idx *index.ChecksumIndex,
	localFileSize,
	matcherCount int64,
	blocksize uint,
) (*comparer.MatchMerger, *comparer.Comparer) {
	// Note: Since not all sections of the file are equal in work
	// it would be better to divide things up into more sections and
	// pull work from a queue channel as each finish
	sectionSize := localFileSize / matcherCount
	sectionSize += int64(blocksize) - (sectionSize % int64(blocksize))
	merger := &comparer.MatchMerger{}
	compare := &comparer.Comparer{}

	for i := int64(0); i < matcherCount; i++ {
		offset := sectionSize * i

		// Sections must overlap by blocksize (strictly blocksize - 1?)
		if i > 0 {
			offset -= int64(blocksize)
		}

		sectionReader := bufio.NewReaderSize(
			io.NewSectionReader(localFile, offset, sectionSize),
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
