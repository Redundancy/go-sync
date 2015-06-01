Go-Sync
------
[![Build Status](https://travis-ci.org/Redundancy/go-sync.svg?branch=master)](https://travis-ci.org/Redundancy/go-sync)
[![GoDoc](https://godoc.org/github.com/Redundancy/go-sync?status.svg)](https://godoc.org/github.com/Redundancy/go-sync)

gosync is a library inspired by zsync and rsync.
Here are the goals:

### Fast
Using the concurrency and performance features of Golang, Go-sync is designed to take advantage of multiple processors and multiple HTTP connections to make the most of modern hardware and minimize the impact of the bandwidth latency product.

### Cross Platform
Works on Windows and Linux, without cygwin or fuss.

### Easy

A new high-level interface designed to reduce the work of implementing block transfer in your application:
```golang
fs := &BasicSummary{...}

rsync, err := MakeRSync(
    localFilename,
    referencePath,
    outFilename,
    fs,
)

if err != nil {
    return err
}

err = rsync.Patch()

if err != nil {
    return err
}

return rsync.Close()
```

### Extensible
All functionality is based on interfaces, allowing customization of behavior:

```golang
// Here, the input version is a local string
inputFile := bytes.NewReader(localVersionAsBytes)

// And the output is a buffer
patchedFile := bytes.NewBuffer(nil)

// This information is meta-data on the file that should be loaded / provided
// You can also provide your own implementation of the FileSummary interface
summary := &BasicSummary{
    ChecksumIndex:  referenceFileIndex,
    // Disable verification of hashes for downloaded data (not really a good idea!)
    ChecksumLookup: nil,
    BlockCount:     uint(blockCount),
    BlockSize:      blockSize,
    FileSize:       int64(len(referenceAsBytes)),
}

rsync := &RSync{
    Input:  inputFile,
    Output: patchedFile,
    // An in-memory block source
    Source: blocksources.NewReadSeekerBlockSource(
        bytes.NewReader(referenceAsBytes),
        blocksources.MakeNullFixedSizeResolver(uint64(blockSize)),
    ),
    Index:   summary,
    Summary: summary,
    OnClose: nil,
}
```

Reuse low level objects to build a new high level library, or implement a new lower-level object to add a new transfer protocol (for example).

### Tested
GoSync has been built from the ground up with unit tests.
The GoSync command-line tool has acceptance tests, although not everything is covered.

## Current State
Go-Sync is still probably not ready for production.

The most obvious areas that still need improvement are the acceptance tests, the error messages,
compression on the blocks that are retrieved from the source and handling of file flags.

### TODO
- [ ] gzip source blocks (this involves writing out a version of the file that's compressed in block-increments)
- [ ] Clean up naming consistency and clarity: Block / Chunk etc
- [ ] Flesh out full directory build / sync
- [ ] Implement 'patch' payloads from a known start point to a desired end state
- [ ] Validate full file checksum after patching
- [ ] Provide bandwidth limiting / monitoring as part of http blocksource
- [ ] Think about turning the filechecksum into an interface
- [ ] Avoid marshalling / un-marshalling blocks during checksum generation
- [ ] Sequential patcher to resume after error?

### Testing

All tests are run by Travis-CI

#### Unit tests

    go test github.com/Redundancy/go-sync/...

#### Acceptance Tests
See the "acceptancetests" folder. This is currently difficult to run locally and relies on several linux utilities.

#### Commandline & files

	go build github.com/Redundancy/go-sync/gosync
	gosync build filenameToPatchTo
	gosync patch filenameToPatchFrom filenameToPatchTo.gosync filenameToPatchTo

Note that normally, patching would rely on a remote http/https file source.

#### Command line tool reference
    gosync --help
