/*
Package gosync is inspired by zsync, and rsync. It aims to take the fundamentals and create a very flexible library that can be adapted
to work in many ways.

We rely heavily on built in Go abstractions like io.Reader, hash.Hash and our own interfaces - this makes the code easier to change, and to test.
In particular, no part of the core library should know anything about the transport or layout of the reference data. If you want
to do rsync and do http/https range requests, that's just as good as zsync client-server over an SSH tunnel. The goal is also to allow
support for multiple concurrent connections, so that you can make the best use of your line in the face of the bandwidth latency product
(or other concerns that require concurrency to solve).

The following optimizations are possible:
* Generate hashes with multiple threads (both during reference generation and local file interrogation)
* Multiple ranged requests (can even be used to get the hashes)

*/
package gosync
