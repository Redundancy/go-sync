Go-Sync
------

gosync is a library inspired by zsync and rsync.

While rsync is built to transfer data between a client and server, zsync is modified to allow distribution over http without an active server (therefore allowing mass-distribution and CDNs). In many ways, it's better than bit-torrent for patching, because it handles offset data blocks due to pre-pending or inserting.

However, in working with these libraries, I've found that they have limitations:
* They generally assume smaller payloads (ISO scale) and don't make full use of available CPUs and IOPs
* The code is pretty intertwined and stateful, making it difficult to modify to optimize
* They require cygwin on windows
* They don't allow incremental updates of directory / zip payloads based on what has changed
* They don't seem to deal with the issue of the tcp bandwidth latency product (see various projects that put UDP underneath)
* HTTP 1.1 support (ranged requests) can still be problematic to end-users
* It's not written to be easily extensible and modifiable for varied uses

I'm not interested in writing a fair UDP file transfer protocol (some companies make a lot of money doing that with WAN optimizers). I am interested in getting updates for big files from A to [B, C, D, ...] across a latent connection as fast as possible, where IOPS and CPU on either end are more economically available than a huge fat pipe and moving the whole lot every time on multiple connections.

The end-goal for this library is to provide a series of composable packages that can be used to implement almost any scheme that uses these patching mechanisms.

The following use cases are foremost in my mind:
* CDN distribution of a directory of files, using https for hashes and the index, and http for content (with checksum verification) [Note that HTTPS can be more expensive on some CDN providers]
* SSH tunneled patching with client/server
 
However, this is also intended to be useable as a library that can be integrated with other systems.

Ideas for extensions or usage could include:
* Optimizing archive storage of many versions of the same file
* Updating distributed caches of files with a single smaller payload by distributing "patch" payloads from known, earlier versions.
* Distributing Docker layers which update existing files, or new versions of images.

### Current State

go-sync is patching "files" in memory in tests and examples. Benchmarks of various elements of index performance are on the agenda as soon as basic functionality is all working as intended.

The commandline tools are currently fleshed out only to the extent that I can test some things. In particular, I'm doing some artificial tests comparing files to themselves, and diffing versions of things like jQuery. 

Duplicated blocks are now being handled (from initial appearances) consistently and correctly now. Performance is pretty good when files are similar (it's almost instant on a same-to-same comparison of around 8 MB). Comparison of jQuery 1.11.1 to 1.2.1 is showing a difference of around 30kiB, with ~200kiB matched at 20b block size.

The rolling checksum should be pretty performant in all forms, as long as it can avoid all allocations (particularly watch out for Sum() if you use that)

After getting on-disk patching working, the next big thing will be to do an http/s blocksource.

### Performance
NB: Based on very rough local testing

We can do about 16 MB/s of rollsum checksums (with a checksum per byte). In comparison, we do ~300 MB/s with a checksum per block of over 100 bytes. (See the rollsum benchmarks)  

However, this isn't the full picture when it comes to evaluating a file:

When the file is very similar to the reference file (lots of matches) we get much better performance - this is because we're hitting closer to the 300 MB/s hash performance, and only looking up 1 weak checksum per block.

When the file isn't very similar, things take much longer (50s vs 200ms) in a comparison that I did... since we're not finding many matching blocks, it's not the merging (that would be more of an issue with the similar file). The file was ~8 MB, so we weren't maxing out at 16 MB/s, and the disk wasn't being taxed at the time.

The conclusion is that the index lookups are likely to be the significant bottleneck in the comparison, even when allowing multiple threads to read the map simultaneously. However, it's quite difficult to benchmark the index without semi-real behaviour of things like the checksums to see where it's worth focussing effort.

### Testing

#### Unit tests

    go test github.com/Redundancy/go-sync/...

#### Commandline & files

	go build github.com/Redundancy/go-sync/gosync
	gosync b filenameToPatchTo
	gosync p filenameToPatchFrom filenameToPatchTo.gosync