package main

import (
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/codegangsta/cli"
	"os"
)

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:      "build",
			ShortName: "b",
			Usage:     "build a .gosync file for a file",
			Action:    Build,
			Flags: []cli.Flag{
				cli.IntFlag{"blocksize", "s", DEFAULT_BLOCK_SIZE, "The block size to use for the gosync file"},
			},
		},
	)
}

func Build(c *cli.Context) {
	filename := c.Args()[0]
	blocksize := c.Int("blocksize")
	generator := filechecksum.NewFileChecksumGenerator(blocksize)

	if f, err := os.Open(filename); err == nil {

	} else {
		switch {
		case os.IsNotExist(err):

		case os.IsPermission(err):

		}
	}

}
