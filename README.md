Go-Sync
------

gosync is a library inspired by zsync and rsync.

While zsync is built to transfer data between a client and server, rsync is modified to allow distribution over http without an active server (therefore allowing mass-distribution and CDNs). In many ways, it's better than bit-torrent for patching, because it handles offset data blocks due to pre-pending or inserting.

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

It's currently capable of identifying identical sections of files (see gosync_test.go for an example). Based on this, it should be relatively easy to ask for different sections and produce a new version of the file based on the combined images.
The new C2 circular buffer implementation is profiling fairly well, with no allocations in the heavy loops and decent performance compared to running md5 on every block.

The next job is to finish off the remaining logic required to take the pieces you already have, a source for the bits you don't, and a place to write the result, to actually reconstruct the reference file.

I expect a lot of structural changes as I feel out the shape of things and try and make sure that the direction of dependencies etc makes sense. This isn't currently in a usable state.