package index

import (
	"github.com/Redundancy/go-sync/chunks"
	"math/rand"
	"sort"
	"testing"
)

var T = []byte{1, 2, 3, 4}

func BenchmarkIndex1024(b *testing.B) {
	i := ChecksumIndex{}
	i.weakChecksumLookup = make([]map[uint32]StrongChecksumList, 256)

	for x := 0; x < 1024; x++ {
		w := rand.Uint32()

		if i.weakChecksumLookup[w&255] == nil {
			i.weakChecksumLookup[w&255] = make(map[uint32]StrongChecksumList)
		}

		i.weakChecksumLookup[w&255][w] = append(
			i.weakChecksumLookup[w&255][w],
			chunks.ChunkChecksum{},
		)
	}

	b.SetBytes(1)
	b.StartTimer()
	for x := 0; x < b.N; x++ {
		i.FindWeakChecksum2(T)
	}
	b.StopTimer()

}

func BenchmarkIndex8192(b *testing.B) {
	i := ChecksumIndex{}
	i.weakChecksumLookup = make([]map[uint32]StrongChecksumList, 256)

	for x := 0; x < 8192; x++ {
		w := rand.Uint32()

		if i.weakChecksumLookup[w&255] == nil {
			i.weakChecksumLookup[w&255] = make(map[uint32]StrongChecksumList)
		}

		i.weakChecksumLookup[w&255][w] = append(
			i.weakChecksumLookup[w&255][w],
			chunks.ChunkChecksum{},
		)
	}

	b.SetBytes(1)
	b.StartTimer()
	for x := 0; x < b.N; x++ {
		i.FindWeakChecksum2(T)
	}
	b.StopTimer()
}

// Check how fast a sorted list of 8192 items would be
func BenchmarkIndexAsListBinarySearch8192(b *testing.B) {
	b.SkipNow()

	s := make([]int, 8192)
	for x := 0; x < 8192; x++ {
		s[x] = rand.Int()
	}

	sort.Ints(s)

	b.StartTimer()
	for x := 0; x < b.N; x++ {
		sort.SearchInts(s, rand.Int())
	}
	b.StopTimer()
}

// Check how fast a sorted list of 8192 items would be
// Checking for cache coherency gains
func BenchmarkIndexAsListLinearSearch8192(b *testing.B) {
	s := make([]int, 8192)
	for x := 0; x < 8192; x++ {
		s[x] = rand.Int()
	}

	sort.Ints(s)

	l := len(s)
	b.StartTimer()
	for x := 0; x < b.N; x++ {
		v := rand.Int()
		for i := 0; i < l; i++ {
			if v < s[i] {
				break
			}
		}
	}
	b.StopTimer()
}

func Benchmark_256SplitBinarySearch(b *testing.B) {
	a := make([][]int, 256)
	for x := 0; x < 8192; x++ {
		i := rand.Int()
		a[i&255] = append(
			a[i&255],
			i,
		)
	}

	for x := 0; x < 256; x++ {
		sort.Ints(a[x])
	}

	b.StartTimer()
	for x := 0; x < b.N; x++ {
		v := rand.Int()
		sort.SearchInts(a[v&255], v)
	}
	b.StopTimer()
}

/*
This is currently the best performing contender for the index data structure for
weak checksum lookups.
*/
func Benchmark_256Split_Map(b *testing.B) {
	a := make([]map[int]interface{}, 256)
	for x := 0; x < 8192; x++ {
		i := rand.Int()
		if a[i&255] == nil {
			a[i&255] = make(map[int]interface{})
		}
		a[i&255][i] = nil
	}

	b.StartTimer()
	for x := 0; x < b.N; x++ {
		v := rand.Int()
		if _, ok := a[v&255][v]; ok {

		}
	}
	b.StopTimer()
}
