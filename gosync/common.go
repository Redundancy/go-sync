package main

import (
	"fmt"
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

func handleFileError(filename string, err error) {
	switch {
	case os.IsNotExist(err):
		fmt.Fprintf(
			os.Stderr,
			"Could not find %v: %v\n",
			filename,
			err,
		)
	case os.IsPermission(err):
		fmt.Fprintf(
			os.Stderr,
			"Could not open %v (permission denied): %v\n",
			filename,
			err,
		)
	default:
		fmt.Fprintf(
			os.Stderr,
			"Unknown error opening %v: %v\n",
			filename,
			err,
		)
	}
}
