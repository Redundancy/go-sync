package main

import (
	"fmt"
	"os"
	"runtime"

	gosync_main "github.com/Redundancy/go-sync"
	"github.com/urfave/cli/v2"
)

const usage = "gosync patch <localfile> <reference index> <reference source> [<output>]"

func init() {
	app.Commands = append(
		app.Commands,
		&cli.Command{
			Name:      "patch",
			Aliases: []string{"p"},
			Usage:     usage,
			Description: `Recreate the reference source file, using an index and a local file that is believed to be similar.
The index should be produced by "gosync build".

<reference index> is a .gosync file and may be a local, unc network path or http/https url
<reference source> is corresponding target and may be a local, unc network path or http/https url
<output> is optional. If not specified, the local file will be overwritten when done.`,
			Action: Patch,
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:  "p",
					Value: runtime.NumCPU(),
					Usage: "The number of streams to use concurrently",
				},
			},
		},
	)
}

// Patch a file
func Patch(c *cli.Context) error {
	errorWrapper(c, func(c *cli.Context) error {

		fmt.Fprintln(os.Stderr, "Starting patching process")

		if l := c.Args().Len(); l < 3 || l > 4 {
			return fmt.Errorf(
				"Usage is \"%v\" (invalid number of arguments)",
				usage,
			)
		}

		localFilename := c.Args().Get(0)
		summaryFile := c.Args().Get(1)
		referencePath := c.Args().Get(2)

		outFilename := localFilename
		if c.Args().Len() == 4 {
			outFilename = c.Args().Get(3)
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
	return nil
}
