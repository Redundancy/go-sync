package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	sync_index "github.com/Redundancy/go-sync/index"
	"github.com/codegangsta/cli"
	"io"
	"os"
	"runtime"
	"time"
)

const (
	KB = 1024
	MB = 1024 * KB
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
				cli.IntFlag{"p", runtime.NumCPU(), "The number of streams to use concurrently"},
			},
		},
	)
}

func Patch(c *cli.Context) {
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

	e := binary.Read(reference_file, binary.LittleEndian, &blocksize)

	if e != nil {
		fmt.Printf("Error loading index: %v", e)
		os.Exit(1)
	}

	fmt.Println("Blocksize: ", blocksize)
	generator := filechecksum.NewFileChecksumGenerator(uint(blocksize))

	readChunks, err := chunks.LoadChecksumsFromReader(
		reference_file,
		generator.WeakRollingHash.Size(),
		generator.StrongHash.Size(),
	)

	if err != nil {
		fmt.Println(err)
		return
	}

	index := sync_index.MakeChecksumIndex(readChunks)
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

	sectionSize := local_file_size / num_matchers
	sectionSize += int64(blocksize) - (sectionSize % int64(blocksize))
	merger := &comparer.MatchMerger{}
	compare := &comparer.Comparer{}
	fmt.Printf("Using %v cores\n", num_matchers)

	for i := int64(0); i < num_matchers; i++ {
		offset := sectionSize * i

		// Sections must overlap by blocksize
		if i > 0 {
			offset -= int64(blocksize)
		}

		sectionReader := bufio.NewReaderSize(
			io.NewSectionReader(local_file, offset, sectionSize),
			MB,
		)

		sectionGenerator := filechecksum.NewFileChecksumGenerator(uint(blocksize))

		matchStream := compare.StartFindMatchingBlocks(
			sectionReader, offset, sectionGenerator, index)

		merger.StartMergeResultStream(matchStream, int64(blocksize))
	}

	mergedBlocks := merger.GetMergedBlocks()

	fmt.Println("\nMatched:")
	totalMatchingSize := uint64(0)
	matchedBlockCountAfterMerging := uint(0)

	for _, b := range mergedBlocks {
		//fmt.Printf("%#v\n", b)
		totalMatchingSize += uint64(b.EndBlock-b.StartBlock+1) * uint64(blocksize)
		matchedBlockCountAfterMerging += b.EndBlock - b.StartBlock + 1
	}

	fmt.Println("Comparisons:", compare.Comparisons)
	fmt.Println("Weak hash hits:", compare.WeakHashHits)
	fmt.Println("Weak hit rate:", 100.0*float64(compare.WeakHashHits)/float64(compare.Comparisons))

	fmt.Println("Strong hash hits:", compare.StrongHashHits)
	fmt.Println("Weak hash error rate:", 100.0*float64(compare.WeakHashHits-compare.StrongHashHits)/float64(compare.WeakHashHits))
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

	fmt.Println("Total missing bytes:", totalMissingSize)
	fmt.Println("Time taken:", time.Now().Sub(start_time))
}
