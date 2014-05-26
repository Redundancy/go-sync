/*
Package index provides the functionality to describe a reference 'file' and its contents in terms of
the weak and strong checksums, in such a way that you can check if a weak checksum is present,
then check if there is a strong checksum that matches.

It also allows lookups in terms of block offsets, so that upon finding a match, you can more efficiently
check if the next block follows it.

The index structure does not lend itself to being an interface - the pattern of taking the result of looking for
the weak checksum and looking up the strong checksum in that requires us to return an object matching an interface which
both packages must know about.

Here is a potential alternative:
Have FindWeakChecksum return an interface{}. If nil, the weak checksum was not found.
Have FindStrongChecksum accept an interface{}. This should be the value from the weak checksum lookup.

This allows the implementation to rely on a previously generated value, without the users knowing what it is.
This breaks the dependency that requires so many packages to import index.
*/
package index

import (
	"bytes"
	"encoding/binary"
	"github.com/Redundancy/go-sync/chunks"
	"sort"
)

type ChecksumIndex struct {
	BlockCount int
	// Find a matching weak checksum, see if there's a matching strong checksum
	weakChecksumLookup map[uint32]StrongChecksumList
}

// Builds an index in which chunks can be found, with their corresponding offsets
// We use this for the
func MakeChecksumIndex(checksums []chunks.ChunkChecksum) *ChecksumIndex {
	n := &ChecksumIndex{
		BlockCount:         len(checksums),
		weakChecksumLookup: make(map[uint32]StrongChecksumList, len(checksums)),
	}

	for _, chunk := range checksums {
		weakChecksumAsString := binary.LittleEndian.Uint32(chunk.WeakChecksum)

		n.weakChecksumLookup[weakChecksumAsString] = append(
			n.weakChecksumLookup[weakChecksumAsString],
			chunk,
		)

	}

	for _, c := range n.weakChecksumLookup {
		sort.Sort(c)
	}

	return n
}

func (index *ChecksumIndex) WeakCount() int {
	return len(index.weakChecksumLookup)
}

func (index *ChecksumIndex) FindWeakChecksumInIndex(weak []byte) StrongChecksumList {
	return index.weakChecksumLookup[binary.LittleEndian.Uint32(weak)]
}

type StrongChecksumList []chunks.ChunkChecksum

// Sortable interface
func (s StrongChecksumList) Len() int {
	return len(s)
}

// Sortable interface
func (s StrongChecksumList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Sortable interface
func (s StrongChecksumList) Less(i, j int) bool {
	return bytes.Compare(s[i].StrongChecksum, s[j].StrongChecksum) == -1
}

func (s StrongChecksumList) FindStrongChecksum(strong []byte) (result []chunks.ChunkChecksum) {
	n := len(s)

	// average length is 1, so fast path comparison
	if n == 1 {
		if bytes.Compare(s[0].StrongChecksum, strong) == 0 {
			return s
		} else {
			return nil
		}
	}

	// find the first possible occurance
	first_gte_checksum := sort.Search(
		n,
		func(i int) bool {
			return bytes.Compare(s[i].StrongChecksum, strong) >= 0
		},
	)

	// out of bounds
	if first_gte_checksum == -1 || first_gte_checksum == n {
		return nil
	}

	// Somewhere in the middle, but the next one didn't match
	if bytes.Compare(s[first_gte_checksum].StrongChecksum, strong) != 0 {
		return nil
	}

	end := first_gte_checksum + 1
	for end < n {
		if bytes.Compare(s[end].StrongChecksum, strong) == 0 {
			end += 1
		} else {
			break
		}

	}

	return s[first_gte_checksum:end]
}
