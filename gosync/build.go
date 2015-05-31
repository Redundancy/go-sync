package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Redundancy/go-sync/filechecksum"
	"github.com/codegangsta/cli"
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
				cli.IntFlag{
					Name:  "blocksize",
					Value: DefaultBlockSize,
					Usage: "The block size to use for the gosync file",
				},
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

	s, _ := inputFile.Stat()
	// TODO: Error?
	file_size := s.Size()

	defer inputFile.Close()

	ext := filepath.Ext(filename)
	outfilePath := filename[:len(filename)-len(ext)] + ".gosync"
	outputFile, err := os.Create(outfilePath)

	if err != nil {
		handleFileError(outfilePath, err)
		os.Exit(1)
	}

	defer outputFile.Close()

	if err = writeHeaders(
		outputFile,
		magicString,
		blocksize,
		file_size,
		[]uint16{majorVersion, minorVersion, patchVersion},
	); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error getting file info: %v\n",
			filename,
			err,
		)
		os.Exit(2)
	}

	start := time.Now()
	_, err = generator.GenerateChecksums(inputFile, outputFile)
	end := time.Now()

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error generating checksum: %v\n",
			filename,
			err,
		)
		os.Exit(2)
	}

	inputFileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error getting file info: %v\n",
			filename,
			err,
		)
		os.Exit(2)
	}

	fmt.Fprintf(
		os.Stderr,
		"Index for %v file generated in %v (%v bytes/S)\n",
		inputFileInfo.Size(),
		end.Sub(start),
		float64(inputFileInfo.Size())/end.Sub(start).Seconds(),
	)
}
