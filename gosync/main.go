/*
gosync is a command-line implementation of the gosync package functionality, primarily as a demonstration of usage
but supposed to be functional in itself.
*/
package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/codegangsta/cli"
)

const (
	DEFAULT_BLOCK_SIZE = 8192
)

var app *cli.App = cli.NewApp()

func main() {
	app.Name = "gosync"
	app.Usage = "Build indexes, patches, patch files"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "profile",
			Usage: "enable HTTP profiling",
		},
		cli.IntFlag{
			Name:  "profilePort",
			Value: 6060,
			Usage: "The number of streams to use concurrently",
		},
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	app.Before = func(c *cli.Context) error {
		if c.Bool("profile") {
			port := fmt.Sprint(c.Int("profilePort"))

			go func() {
				log.Println(http.ListenAndServe("localhost:"+port, nil))
			}()
		}

		return nil
	}

	app.Run(os.Args)
}
