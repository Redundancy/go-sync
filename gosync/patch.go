package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/Redundancy/go-sync/chunks"
	"github.com/Redundancy/go-sync/comparer"
	"github.com/Redundancy/go-sync/filechecksum"
	sync_index "github.com/Redundancy/go-sync/index"
	"github.com/Redundancy/go-sync/patcher/sequential"
	"github.com/Redundancy/go-sync/util/blocksources"
	"github.com/codegangsta/cli"
	"io"
	"io/ioutil"
	"os"
	"runtime"
)

func init() {
	app.Commands = append(
		app.Commands,
		cli.Command{
			Name:      "patch",
			ShortName: "p",
			Usage:     "gosync patch <localfile> <reference index> <reference source> [<output>]",
			Description: `Recreate the reference source file, using an index and a local file that is believed to be similar.
The index should be produced by "gosync build". 

<reference index> is a .gosync file and may be a local, unc network path or http/https url
<reference source> is corresponding target and may be a local, unc network path or http/https url
<output> is optional. If not specified, the local file will be overwritten when done.`,
			Action: Patch,
			Flags: []cli.Flag{
				cli.IntFlag{"p", runtime.NumCPU(), "The number of streams to use concurrently"},
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

	// hello
	err = binary.Read(indexReader, binary.LittleEndian, &blocksize)

	if err != nil {
		return
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

	// TODO: is source is a local file, use the reader block source
	source := blocksources.NewHttpBlockSource(reference_arg, 4)

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
