package main

import (
	"fmt"
	"os"
	"runtime"

	gosync_main "github.com/Redundancy/go-sync"
	"github.com/codegangsta/cli"
)

const usage = "gosync patch <localfile> <reference index> <reference source> [<output>]"

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:      "patch",
			ShortName: "p",
			Usage:     usage,
			Description: `Recreate the reference source file, using an index and a local file that is believed to be similar.
The index should be produced by "gosync build".

<reference index> is a .gosync file and may be a local, unc network path or http/https url
<reference source> is corresponding target and may be a local, unc network path or http/https url
<output> is optional. If not specified, the local file will be overwritten when done.`,
			Action: Patch,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "p",
					Value: runtime.NumCPU(),
					Usage: "The number of streams to use concurrently",
				},
			},
		},
	)
}

// Patch a file
func Patch(c *cli.Context) {
	errorWrapper(c, func(c *cli.Context) error {

		fmt.Fprintln(os.Stderr, "Starting patching process")

		if l := len(c.Args()); l < 3 || l > 4 {
			return fmt.Errorf(
				"Usage is \"%v\" (invalid number of arguments)",
				usage,
			)
		}

		localFilename := c.Args()[0]
		summaryFile := c.Args()[1]
		referencePath := c.Args()[2]

		outFilename := localFilename
		if len(c.Args()) == 4 {
			outFilename = c.Args()[3]
		}

		indexReader, e := os.Open(summaryFile)
		if e != nil {
			return e
		}
		defer indexReader.Close()

		_, _, _, filesize, blocksize, e := readHeadersAndCheck(
			indexReader,
			magicString,
			majorVersion,
		)

		index, checksumLookup, blockCount, err := readIndex(
			indexReader,
			uint(blocksize),
		)

		fs := &gosync_main.BasicSummary{
			ChecksumIndex:  index,
			ChecksumLookup: checksumLookup,
			BlockCount:     blockCount,
			BlockSize:      uint(blocksize),
			FileSize:       filesize,
		}

		rsync, err := gosync_main.MakeRSync(
			localFilename,
			referencePath,
			outFilename,
			fs,
		)

		if err != nil {
			return err
		}

		err = rsync.Patch()

		if err != nil {
			return err
		}

		return rsync.Close()
	})
}
