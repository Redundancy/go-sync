package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/Redundancy/go-sync/blocksources"
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
	fmt.Fprintln(os.Stderr, "Starting patching process")
	var err error = nil

	// handle error cases and exit as the last thing we do
	// don't use os.Exit elsewhere, let defer handlers clean up
	defer func() {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	if l := len(c.Args()); l < 3 || l > 4 {
		fmt.Fprintf(os.Stderr, "Usage is \"%v\" (invalid number of arguments)", USAGE)
		return
	}

	local_filename := c.Args()[0]
	gosync_arg := c.Args()[1]
	reference_arg := c.Args()[2]

	var local_file_closed bool
	local_file, err := os.Open(local_filename)

	if err != nil {
		err = formatFileError(local_filename, err)
		return
	}

	defer func() {
		if e := local_file.Close(); err == nil && !local_file_closed {
			err = e
		}
	}()

	var out_file *os.File = nil

	out_filename := local_filename
	if len(c.Args()) == 4 {
		out_filename = c.Args()[3]
	}

	fmt.Fprintln(os.Stderr, "Output file is", out_filename)

	use_temp_file := false
	if same, err := is_same_file(out_filename, local_filename); err != nil {
		return
	} else if same {
		use_temp_file = true
	}

	temp_file_name := ""
	if use_temp_file {
		out_file, err = ioutil.TempFile(".", "tmp_")
		temp_file_name = out_file.Name()
		fmt.Printf("Using temporary file: %v\n", temp_file_name)
	} else if _, err := os.Stat(out_filename); os.IsNotExist(err) {
		out_file, err = os.Create(out_filename)
		fmt.Printf("Creating file: %v\n", out_filename)
	} else {
		out_file, err = os.OpenFile(out_filename, os.O_WRONLY, 0)
		fmt.Printf("Using existing file: %v\n", out_filename)
	}

	if err != nil {
		return
	}

	var blocksize uint32
	indexReader, err := getLocalOrRemoteFile(gosync_arg)

	if err != nil {
		err = formatFileError(local_filename, err)
		return
	}

	_, _, _, filesize, blocksize, e := read_headers_and_check(indexReader, magic_string, major_version)

	if e != nil {
		indexReader.Close()
		fmt.Printf("Error loading index: %v", e)
		os.Exit(1)
	}

	index, err := read_index(indexReader, uint(blocksize))
	indexReader.Close()

	if err != nil {
		return
	}

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

	merger, _ := multithreaded_matching(
		local_file,
		index,
		local_file_size,
		num_matchers,
		uint(blocksize),
	)

	mergedBlocks := merger.GetMergedBlocks()
	missing := mergedBlocks.GetMissingBlocks(uint(index.BlockCount) - 1)

	var source *blocksources.BlockSourceBase
	resolver := blocksources.MakeFileSizedBlockResolver(
		uint64(blocksize),
		filesize,
	)

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

		defer f.Close()

		source = blocksources.NewReadSeekerBlockSource(f, resolver)
	}

	if err = sequential.SequentialPatcher(
		local_file,
		source,
		toPatcherMissingSpan(missing, int64(blocksize)),
		toPatcherFoundSpan(mergedBlocks, int64(blocksize)),
		MAX_PATCHING_BLOCK_STORAGE,
		out_file,
	); err != nil {
		err = fmt.Errorf("Error patching to out_file: %v", err)
		return
	}

	//out_file.Close()
	if err = local_file.Close(); err != nil {
		return
	}

	local_file_closed = true

	if use_temp_file {
		// copy to local
		lf, err := os.OpenFile(local_filename, os.O_WRONLY, 0)
		if err != nil {
			return
		}

		defer func() {
			e := lf.Close()
			if err == nil {
				err = e
			}
		}()

		//current, _ := out_file.Seek(1, 0)
		//out_file.Truncate(current)
		//lf.Truncate(current)

		out_file.Seek(0, 0)
		_, err = io.Copy(lf, out_file)

		if err != nil {
			err = fmt.Errorf("Error copying to local file: %v", err)
			return
		}
	}

	if err = out_file.Close(); err != nil {
		err = fmt.Errorf("Error closing out_file: %v", err)
		return
	}

	if use_temp_file {
		os.Remove(temp_file_name)
	}

	fmt.Printf("Downloaded %v bytes\n", source.ReadBytes())
	//fmt.Printf("Total file is %v bytes\n", int64(index.BlockCount)*int64(blocksize))
}
