package main

import (
	"fmt"
	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/codegangsta/cli"
	"os"
	"path/filepath"
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
				cli.IntFlag{"blocksize", DEFAULT_BLOCK_SIZE, "The block size to use for the gosync file"},
			},
		},
	)
}

func Build(c *cli.Context) {
	filename := c.Args()[0]
	blocksize := uint(c.Int("blocksize"))
	generator := filechecksum.NewFileChecksumGenerator(blocksize)

	inputFile, err := os.Open(filename)

	if err != nil {
		absInputPath, err2 := filepath.Abs(filename)
		if err2 == nil {
			handleFileError(absInputPath, err)
		} else {
			handleFileError(filename, err)
		}

		os.Exit(1)
	}

	defer inputFile.Close()

	ext := filepath.Ext(filename)
	outfilePath := filename[:len(filename)-len(ext)] + ".gosync"
	outputFile, err := os.Create(outfilePath)

	if err != nil {
		handleFileError(outfilePath, err)
		os.Exit(1)
	}

	defer outputFile.Close()

	// TODO: write the blocksize first
	//outputFile.Write(binary.LittleEndian.)
	_, err = generator.GenerateChecksums(inputFile, outputFile)

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error generating checksum: %v\n",
			filename,
			err,
		)
		os.Exit(2)
	}
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
