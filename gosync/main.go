/*
gosync is a command-line implementation of the gosync package functionality, primarily as a demonstration of usage
but supposed to be functional in itself.
*/
package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
)

const (
	DEFAULT_BLOCK_SIZE = 8192
)

var app *cli.App = cli.NewApp()

func main() {
	app.Name = "gosync"
	app.Usage = "Build indexes, patches, patch files"

	/*
		// TODO: how to enable profiling?
		// os.Exit will cause the profile to fail to be written

		app.Flags = []cli.Flag{
			cli.BoolFlag{"prof", false, "Output a CPU profile as gosync.pprof"},
		}

		app.Before = func(c *cli.Context) {
			if c.Bool("prof") {

			}
		}
	*/
	runtime.GOMAXPROCS(runtime.NumCPU())
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
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
