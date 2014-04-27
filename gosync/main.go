/*
gosync is a command-line implementation of the gosync package functionality, primarily as a demonstration of usage
but supposed to be functional in itself.
*/
package main

import (
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
