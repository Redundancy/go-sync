Go-Sync
------
[![Build Status](https://travis-ci.org/Redundancy/go-sync.svg?branch=master)](https://travis-ci.org/Redundancy/go-sync)
[![GoDoc](https://godoc.org/github.com/Redundancy/go-sync?status.svg)](https://godoc.org/github.com/Redundancy/go-sync)

gosync is a library inspired by zsync and rsync. The intent is that it's easier to build upon than the zsync/rsync codebases. By writing it in Go, it's easier to create in a way that's cross-platform, can take advantage of multiple CPUs with built in benchmarks, code documentation and unit tests.

There are many areas that benefit from the use of multiple threads & connections:
* Making use of multiple http connections, to avoid the bandwidth latency product limiting transfer rates to remote servers
* Making use of multiple CPUs while doing the comparisons

gosync includes a commandline tool that's intended as a starting point for building the functionality that you want.

Zsync modified rsync to allow it to be used against a "dumb" http server, but we can go further:
* Arrange files by hashed path, and checksum: if a file hasn't changed, you can serve the existing version (NB: this works well with s3 sync)
* Split the checksum blocks from the data: serve checksum blocks securely over https, while allowing caching on the data over http
* Split up the data files: improve the fallback when there's a non HTTP 1.1 proxy between the client and server

### Current State

The command-line tools are fleshed out enough for testing comparison behaviour and profiling it against real input files. 
***NB: The command-line tools are not in a state for use in production!***

There is a basic HTTP Blocksource, which currently supports fixed size blocks (no compression on the source), but which should be able to multiple tcp connections to increase transfer speed where latency is a bottleneck.

Work needs to be done to add support for a BlockSourceResolver that can deal with compressed blocks.
Some changes and refactoring need to happen where there are assumptions about a source block being of 'blocksize' - in order
to optimize transmitted size, blocks should be gziped. Being able to support multiple source files would also be good.

After that, there needs to be some cleanup of the CLI command code, which is pretty verbose and duplicates a lot. Things like version numbers in the files would be good, and then implementation of more features to make it potentially usable.

### Performance
On an 8 MB file with few matches, I'm hitting about 16 MB/s with 4 threads. I think that we're mostly CPU bound, and should scale reasonably well with more processors.
When there are very similar files, the speed is far higher (since the weak checksum match is significantly cheaper)

#### Some numbers:
Budget for 8 MB/s byte by byte comparison on single thread: 120ns

Current Benchmark State (Golang 1.4):
- Checksum: 50.3 ns
- Comparison (No index lookup)
  - Weak Rejection: 68.6 ns
  - Strong Rejection: 326 ns

Generating a gosync file for a 22 GB file (not on an SSD) took me around 2m31s ~= 145 MB/s sustained checksum generation.
The resulting file was around 50 MB and does not compress well (which makes sense, since it's hashes with a hopefully near-random distribution).

The 32 bit Rollsum hash produces far fewer false positives than the 16 bit one, with the same 4 byte hash overhead.

Index generation:
- Easily hits 100 MB/s on my workstation, satisfying the idea that you should be able to build 12 GB payloads in ~1m
- More likely to be bottlenecked by the disk throughput / seek time than the CPU

### TODO
- [ ] gzip source blocks (this involves writing out a version of the file that's compressed in block-increments)
- [x] support variable length source blocks
- [ ] Provide more helpers to make common usage easier (multi-threading etc)
- [ ] Clean up naming consistency and clarity: Block / Chunk etc
- [ ] Flesh out full directory build / sync
- [ ] Implement 'patch' payloads from a known start point to a desired endstate
- [ ] Validate full file checksum after patching
- [ ] Provide bandwidth limiting / monitoring as part of http blocksource
- [ ] Think about turning the filechecksum into an interface
- [ ] Avoid marshalling / unmarshalling blocks during checksum generation
- [ ] Sequential patcher to resume after error?

### Testing

#### Unit tests

    go test github.com/Redundancy/go-sync/...

#### Commandline & files

	go build github.com/Redundancy/go-sync/gosync
	gosync build filenameToPatchTo
	gosync patch filenameToPatchFrom filenameToPatchTo.gosync filenameToPatchTo

Note that normally, patching would rely on a remote http/https file source.
