package main

import (
	"fmt"
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	sync_index "github.com/Redundancy/go-sync/index"
	"github.com/codegangsta/cli"
	"os"
)

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:      "patch",
			ShortName: "p",
			Usage:     "gosync patch <localfile> <reference.gosync>",
			Description: `
			
			`,
			Action: Patch,
			Flags: []cli.Flag{
				cli.IntFlag{"blocksize", DEFAULT_BLOCK_SIZE, "The block size to use for the gosync file"},
			},
		},
	)
}

func Patch(c *cli.Context) {
	local_filename := c.Args()[0]
	reference_filename := c.Args()[1]

	blocksize := uint(c.Int("blocksize"))
	generator := filechecksum.NewFileChecksumGenerator(blocksize)

	local_file := openFileAndHandleError(local_filename)

	if local_file == nil {
		os.Exit(1)
	}

	defer local_file.Close()

	reference_file := openFileAndHandleError(reference_filename)

	if reference_file == nil {
		os.Exit(1)
	}

	defer reference_file.Close()

	fmt.Println("Loading checksums")
	readChunks, err := chunks.LoadChecksumsFromReader(
		reference_file,
		generator.WeakRollingHash.Size(),
		generator.StrongHash.Size(),
	)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("building index")
	index := sync_index.MakeChecksumIndex(readChunks)

	fmt.Println("Finding matching blocks")
	matchStream := comparer.FindMatchingBlocks(local_file, 0, generator, index)

	merger := &comparer.MatchMerger{}
	fmt.Println("Merging")
	merger.MergeResults(matchStream, int64(blocksize))

	mergedBlocks := merger.GetMergedBlocks()

	totalMatchingSize := uint64(0)
	for _, b := range mergedBlocks {
		fmt.Printf("%#v\n", b)
		totalMatchingSize += uint64(b.EndBlock-b.StartBlock+1) * uint64(blocksize)
	}

	fmt.Println("Total matched bytes:", totalMatchingSize)
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
