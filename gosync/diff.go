package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	"runtime"
	"time"
)

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:        "diff",
			ShortName:   "d",
			Usage:       "gosync diff <localfile> <reference.gosync>",
			Description: `Compare a file with a reference index, and print statistics on the comparison and performance.`,
			Action:      Diff,
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

func Diff(c *cli.Context) {
	local_filename := c.Args()[0]
	reference_filename := c.Args()[1]
	start_time := time.Now()

	local_file := openFileAndHandleError(local_filename)

	if local_file == nil {
		os.Exit(1)
	}

	defer local_file.Close()

	var blocksize uint32
	reference_file := openFileAndHandleError(reference_filename)

	if reference_file == nil {
		os.Exit(1)
	}

	defer reference_file.Close()

	_, _, _, _, blocksize, e := read_headers_and_check(reference_file, magic_string, major_version)

	if e != nil {
		fmt.Printf("Error loading index: %v", e)
		os.Exit(1)
	}

	fmt.Println("Blocksize: ", blocksize)

	index, err := read_index(reference_file, uint(blocksize))
	reference_file.Close()

	if err != nil {
		return
	}

	fmt.Println("Weak hash count:", index.WeakCount())

	fi, err := local_file.Stat()

	if err != nil {
		fmt.Println("Could not get info on file:", err)
		os.Exit(1)
	}

	num_matchers := int64(c.Int("p"))

	local_file_size := fi.Size()

	// Don't split up small files
	if local_file_size < 1024*1024 {
		num_matchers = 1
	}

	merger, compare := multithreaded_matching(
		local_file,
		index,
		local_file_size,
		num_matchers,
		uint(blocksize),
	)

	mergedBlocks := merger.GetMergedBlocks()

	fmt.Println("\nMatched:")
	totalMatchingSize := uint64(0)
	matchedBlockCountAfterMerging := uint(0)

	for _, b := range mergedBlocks {
		totalMatchingSize += uint64(b.EndBlock-b.StartBlock+1) * uint64(blocksize)
		matchedBlockCountAfterMerging += b.EndBlock - b.StartBlock + 1
	}

	fmt.Println("Comparisons:", compare.Comparisons)
	fmt.Println("Weak hash hits:", compare.WeakHashHits)

	if compare.Comparisons > 0 {
		fmt.Printf(
			"Weak hit rate: %.2f%%\n",
			100.0*float64(compare.WeakHashHits)/float64(compare.Comparisons),
		)
	}

	fmt.Println("Strong hash hits:", compare.StrongHashHits)
	if compare.WeakHashHits > 0 {
		fmt.Printf(
			"Weak hash error rate: %.2f%%\n",
			100.0*float64(compare.WeakHashHits-compare.StrongHashHits)/float64(compare.WeakHashHits),
		)
	}

	fmt.Println("Total matched bytes:", totalMatchingSize)
	fmt.Println("Total matched blocks:", matchedBlockCountAfterMerging)

	// TODO: GetMissingBlocks uses the highest index, not the count, this can be pretty confusing
	// Should clean up this interface to avoid that
	missing := mergedBlocks.GetMissingBlocks(uint(index.BlockCount) - 1)
	fmt.Println("Index blocks:", index.BlockCount)

	totalMissingSize := uint64(0)
	for _, b := range missing {
		//fmt.Printf("%#v\n", b)
		totalMissingSize += uint64(b.EndBlock-b.StartBlock+1) * uint64(blocksize)
	}

	fmt.Println("Approximate missing bytes:", totalMissingSize)
	fmt.Println("Time taken:", time.Now().Sub(start_time))
}
