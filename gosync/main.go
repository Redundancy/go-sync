/*
gosync is a command-line implementation of the gosync package functionality, primarily as a demonstration of usage
but supposed to be functional in itself.
*/
package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

const (
	DEFAULT_BLOCK_SIZE = 8096
)

var app *cli.App = cli.NewApp()

func main() {
	app.Name = "gosync"
	app.Usage = "Build indexes, patches, patch files"
	app.Run(os.Args)
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
