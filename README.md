Go-Sync
------
[![Build Status](https://travis-ci.org/Redundancy/go-sync.svg?branch=master)](https://travis-ci.org/Redundancy/go-sync)
[![GoDoc](https://godoc.org/github.com/Redundancy/go-sync?status.svg)](https://godoc.org/github.com/Redundancy/go-sync)

# The Command-line tool has moved!
In order to split issues between the library and the CLI tool, as well as correctly vendor dependencies, the command-line tool code has been moved to its own repository: https://github.com/Redundancy/gosync-cmd

# Why *not* use a Zsync mechanism?

Consider if a binary differential sync mechanism is appropriate to your use case:

The ZSync mechanism has the weakness that HTTP1.1 ranged requests are not always well supported by CDN providers and ISP proxies. When issues happen, they're very difficult to respond to correctly in software (if possible at all). Using HTTP 1.0 and fully completed GET requests would be better, if possible.

There are some other issues too - ZSync doesn't (as far as I'm aware) solve any issues to do with storage of a files, which can get more and more onerous for large files that are not changing much from one version to another.

On a project I worked on, we switched instead to storing individual files that were part of a larger build (like an ISO) by filename and hashes, mainly maintaining an index of which files comprised the full build. By doing this, we significantly decreased the required storage (new files were only required when they changed), allowed multiple versions to sit efficiently side by side and very simple file serving to be used efficiently (with a tiny library to resolve and fetch files).

# The GoSync library

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

**HOWEVER** this library has never been used in production against real-world network problems, and I cannot personally guarantee that it will work as intended.

## Current State
The GoSync library is fairly well unit-tested, but not tested through exposure to real-world network conditions. As an example, the HTTP client used is a default HTTP client, and is therefore lacking decent timeouts. As such, I would not recommend depending on the code in production unless you're willing to validate the results and debug issues like that.

In terms of activity, I have been extremely busy with other things for the last few months, and will continue to be. I do not expect to put a huge amount more work into this, since we solved our problem in a simpler (and significant more elegant) way explained in a section above.

### Request for Enhancement
If the library or tool are still something that you feel would be useful, here are some issues and ideas for work that could be done.

#### GZip support - Performance / Efficiency Enhancement (!)
In order to be more efficient in the transfer of data from the source to the client, gosync should support compressed blocks. This requires changing any assumptions about the offset of a block, and the length of a block to read (especially when merging block ranges), then adding a compression / decompression call to the interfaces.

In terms of the CLI tool, this probably means that gosync should build a version of the source file where each block is independently compressed and store the block-sizes in the index. It can then rebuild the offsets incrementally.

#### Patch payloads - Feature
Given a known original version, and a known desired state, it would be possible to create a "patch", which has enough information to store the required blocks for the transformation only, and only enough of the index to validate that it's transforming the correct file.

#### Patched file Validation - Feature (!)
GoSync should validate the full MD5 and length of a file after it is done with patching it. This should be minimally expensive, and help increase confidence that GoSync has produced the correct result.

This one is pretty simple. :)

#### Network Error handling - Improvement (!!)
The HTTP Blocksource does not handle connection / read timeouts and other myriad possible network failures . Handling these correctly is important to making it robust and production-ready.

Rolled into this is to correctly identify resumable errors (including rate-limiting, try-again-later and temporary errors) and back-off strategies.

#### Rate limiting - Feature
In order to be a good network denizen, GoSync should be able to support rate-limiting.

#### Better / Consistent naming - Improvement
The current naming of some packages and concepts is a bit broken. The RSync object, for example, has nothing to do with RSync. Blocks and Chunks are used interchangeably for a byte range.

### Testing

All tests are run by Travis-CI

#### Unit tests
    go test github.com/Redundancy/go-sync/...
