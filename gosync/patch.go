package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/Redundancy/go-sync/blocksources"
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	sync_index "github.com/Redundancy/go-sync/index"
	"github.com/Redundancy/go-sync/patcher/sequential"
	"github.com/codegangsta/cli"
)

const USAGE = "gosync patch <localfile> <reference index> <reference source> [<output>]"

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:      "patch",
			ShortName: "p",
			Usage:     USAGE,
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

// Making up a number
const MAX_PATCHING_BLOCK_STORAGE = 40

func Patch(c *cli.Context) {
	var err error = nil

	// handle error cases and exit as the last thing we do
	// don't use os.Exit elsewhere, let defer handlers clean up
	defer func() {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	switch len(c.Args()) {
	case 3:
	case 4:
	default:
		fmt.Fprintf(os.Stderr, "Usage is \"%v\" (invalid number of arguments)", USAGE)
		return
	}

	local_filename := c.Args()[0]
	gosync_arg := c.Args()[1]
	reference_arg := c.Args()[2]
	use_tempfile := len(c.Args()) == 4

	local_file, err := os.Open(local_filename)

	if err != nil {
		err = formatFileError(local_filename, err)
		return
	}

	defer func() {
		if e := local_file.Close(); err == nil {
			err = e
		}
	}()

	var out_file *os.File = nil

	if use_tempfile {
		out_file, err = ioutil.TempFile(".", "tmp_")

		defer func() {
			if err == nil {
				tempfilename := out_file.Name()
				finalName := c.Args()[3]

				if _, e := os.Stat(finalName); e != nil && !os.IsNotExist(e) {
					err = e
					return
				} else if e == nil {
					err = os.Remove(finalName)
				}

				if err == nil {
					err = os.Rename(tempfilename, finalName)
				}
			}
		}()
	} else {
		out_file, err = os.Create(c.Args()[3])
		out_filename := out_file.Name()

		// Cleanup the temporary file after it is closed
		defer func() {
			if e := os.Remove(out_filename); err != nil {
				err = e
			}
		}()
	}

	if err != nil {
		return
	}

	defer func() {
		if e := out_file.Close(); err == nil {
			err = e
		}
	}()

	var blocksize uint32
	indexReader, err := getLocalOrRemoteFile(gosync_arg)

	if err != nil {
		err = formatFileError(local_filename, err)
		return
	}

	defer func() {
		if e := indexReader.Close(); err == nil {
			err = e
		}
	}()

	_, _, _, blocksize, e := read_headers_and_check(indexReader, magic_string, major_version)

	if e != nil {
		fmt.Printf("Error loading index: %v", e)
		os.Exit(1)
	}

	generator := filechecksum.NewFileChecksumGenerator(uint(blocksize))

	readChunks, err := chunks.LoadChecksumsFromReader(
		indexReader,
		generator.WeakRollingHash.Size(),
		generator.StrongHash.Size(),
	)

	if err != nil {
		return
	}

	index := sync_index.MakeChecksumIndex(readChunks)

	fi, err := local_file.Stat()

	if err != nil {
		return
	}

	num_matchers := int64(c.Int("p"))

	local_file_size := fi.Size()

	// Don't split up small files
	if local_file_size < MB {
		num_matchers = 1
	}

	// Note: Since not all sections of the file are equal in work
	// it would be better to divide things up into more sections and
	// pull work from a queue channel as each finish
	sectionSize := local_file_size / num_matchers
	sectionSize += int64(blocksize) - (sectionSize % int64(blocksize))
	merger := &comparer.MatchMerger{}
	compare := &comparer.Comparer{}

	for i := int64(0); i < num_matchers; i++ {
		offset := sectionSize * i

		// Sections must overlap by blocksize (strictly blocksize - 1?)
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
	missing := mergedBlocks.GetMissingBlocks(uint(index.BlockCount) - 1)
	var source *blocksources.BlockSourceBase
	resolver := blocksources.MakeNullFixedSizeResolver(uint64(blocksize))

	if strings.HasPrefix(reference_arg, "http://") || strings.HasPrefix(reference_arg, "https://") {
		source = blocksources.NewHttpBlockSource(
			reference_arg,
			4,
			resolver,
		)
	} else {
		f, err := os.Open(reference_arg)

		if err != nil {
			return
		}

		source = blocksources.NewReadSeekerBlockSource(f, resolver)
	}

	err = sequential.SequentialPatcher(
		local_file,
		source,
		toPatcherMissingSpan(missing, int64(blocksize)),
		toPatcherFoundSpan(mergedBlocks, int64(blocksize)),
		MAX_PATCHING_BLOCK_STORAGE,
		out_file,
	)

	fmt.Printf("Downloaded %v bytes\n", source.ReadBytes())
	fmt.Printf("Total file is %v bytes\n", int64(index.BlockCount)*int64(blocksize))

}
