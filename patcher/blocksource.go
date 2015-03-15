/*
Package patcher follows a pattern established by hash, which defines the interface in the top level package, and then provides implementations
below it.
*/
package patcher

import (
	"hash"
)

/*
BlockSource is an interface used by the patchers to obtain blocks from the reference
It does not stipulate where the reference data might be (it could be local, in a pre-built patch file, on S3 or somewhere else)

It is assumed that the BlockSource may be slow, and may benefit from request pipelining & concurrency.
Therefore patchers should feel free to request as many block spans as they can handle.

A BlockSource may be a view onto a larger transport concept, so that multiple files can be handled with wider
knowledge of the number of simultaneous requests allowed, etc. The BlockSource may decide to split BlockSpans
into smaller sizes if it wants.

It is up to the patcher to receive blocks in a timely manner, and decide what to do with them, rather than
bother the BlockSource with more memory management and buffering logic.

Since these interfaces require specific structs to satisfy, it's expected that implementers will import this module.

*/
type BlockSource interface {
	RequestBlocks(MissingBlockSpan) error

	GetResultChannel() <-chan BlockReponse

	// If the block source encounters an unsurmountable problem
	EncounteredError() <-chan error
}

type FoundBlockSpan struct {
	StartBlock  uint
	EndBlock    uint
	BlockSize   int64
	MatchOffset int64
}

type MissingBlockSpan struct {
	StartBlock uint
	EndBlock   uint

	BlockSize  int64
	// a hasher to use to ensure that the block response matches
	Hasher hash.Hash
	// the hash values that the blocks should have
	ExpectedSums [][]byte
}

type BlockReponse struct {
	StartBlock uint
	Data       []byte
}
