package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Redundancy/go-sync/comparer"
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
	if _, err := url.Parse(path); err == nil {
		response, err := http.Get(path)

		if err != nil {
			return nil, err
		}

		if response.StatusCode < 200 || response.StatusCode > 299 {
			return nil, fmt.Errorf("Request to %v returned status: %v", path, response.Status)
		}

		return response.Body, nil
	} else {
		return os.Open(path)
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

func write_headers(f *os.File, magic string, blocksize uint32, versions []uint16) (err error) {
	if _, err = f.WriteString(magic_string); err != nil {
		return
	}

	for _, v := range versions {
		if err = binary.Write(f, binary.LittleEndian, v); err != nil {
			return
		}
	}

	err = binary.Write(f, binary.LittleEndian, blocksize)
	return
}

// reads the file headers and checks the magic string, then the semantic versioning
func read_headers_and_check(r io.Reader, magic string, required_major_version uint16) (major, minor, patch uint16, blocksize uint32, err error) {
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

	err = binary.Read(r, binary.LittleEndian, &blocksize)
	return
}
