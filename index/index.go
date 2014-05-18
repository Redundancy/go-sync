/*
Package index provides the functionality to describe a reference 'file' and its contents in terms of
the weak and strong checksums, in such a way that you can check if a weak checksum is present,
then check if there is a strong checksum that matches.

It also allows lookups in terms of block offsets, so that upon finding a match, you can more efficiently
check if the next block follows it.
*/
package index

import (
	"bytes"
	"github.com/Redundancy/go-sync/chunks"
	"sort"
)

type ChecksumIndex struct {
	BlockCount int
	// Find a matching weak checksum, see if there's a matching strong checksum
	weakChecksumLookup map[string]StrongChecksumList
}

// Builds an index in which chunks can be found, with their corresponding offsets
// We use this for the
func MakeChecksumIndex(checksums []chunks.ChunkChecksum) *ChecksumIndex {
	n := &ChecksumIndex{
		BlockCount:         len(checksums),
		weakChecksumLookup: make(map[string]StrongChecksumList, len(checksums)),
	}

	for _, chunk := range checksums {
		weakChecksumAsString := string(chunk.WeakChecksum)

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

func (index *ChecksumIndex) FindWeakChecksumInIndex(weak []byte) StrongChecksumList {
	return index.weakChecksumLookup[string(weak)]
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
	// find the first possible occurance
	start := sort.Search(
		len(s),
		func(i int) bool { return bytes.Compare(s[i].StrongChecksum, strong) >= 0 },
	)

	end := start + 1
	len_s := len(s)
	for end+1 < len_s {
		if bytes.Compare(s[end].StrongChecksum, strong) == 0 {
			end += 1
		} else {
			break
		}

	}

	return s[start:end]
}
