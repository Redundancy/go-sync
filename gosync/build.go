package main

import (
	"encoding/binary"
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
	blocksize := uint32(c.Int("blocksize"))
	generator := filechecksum.NewFileChecksumGenerator(uint(blocksize))

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

	// Embed the blocksize as a uint32 at the front
	binary.Write(outputFile, binary.LittleEndian, &blocksize)

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
