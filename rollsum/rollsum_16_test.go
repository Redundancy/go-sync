package rollsum

import (
	"bytes"
	"crypto/md5"
	"github.com/Redundancy/go-sync/circularbuffer"
	"hash"
	"io"
	"testing"
)

func TestFULL_BYTES_16Value(t *testing.T) {
	if FULL_BYTES_16 != 65535 {
		t.Errorf("FULL_BYTES_16 does not fill 15 bits of 1 %v", FULL_BYTES_16)
	}
}

func TestThatRollsumSatisfiesHashInterface(t *testing.T) {
	var i hash.Hash = NewRollsum16(10)
	i.Reset()
}

func TestThatRollsumSatisfiedWriterInterface(t *testing.T) {
	var i io.Writer = NewRollsum16(10)
	n, err := i.Write([]byte{1, 2, 3, 4})

	if n != 4 {
		t.Error("Did not report writing 4 bytes")
	}

	if err != nil {
		t.Error(err)
	}
}

func TestThatRollsumIsTheSameAfterBlockSizeBytes(t *testing.T) {
	r1 := NewRollsum16(4)
	r2 := NewRollsum16(4)

	r1.Write([]byte{1, 2, 3, 4})
	r2.Write([]byte{7, 6, 5, 1, 2, 3, 4})

	sum1 := r1.Sum(nil)
	sum2 := r2.Sum(nil)

	if bytes.Compare(sum1, sum2) != 0 {
		t.Errorf(
			"Rollsums are different \"%v\" vs \"%v\"",
			sum1,
			sum2,
		)
	}
}

func TestThatRollsumIsDifferentForDifferentInput(t *testing.T) {
	r1 := NewRollsum16(4)
	r2 := NewRollsum16(4)

	r1.Write([]byte{1, 2, 3, 4})
	r2.Write([]byte{7, 6, 5, 1})

	sum1 := r1.Sum(nil)
	sum2 := r2.Sum(nil)

	if bytes.Compare(sum1, sum2) == 0 {
		t.Errorf(
			"Rollsums should be different \"%v\" vs \"%v\"",
			sum1,
			sum2,
		)
	}
}

func TestResettingTheRollsum(t *testing.T) {
	r1 := NewRollsum16(4)
	r2 := NewRollsum16(4)

	r1.Write([]byte{1, 2, 3})

	r2.Write([]byte{7, 6})
	r2.Reset()
	r2.Write([]byte{1, 2, 3})

	sum1 := r1.Sum(nil)
	sum2 := r2.Sum(nil)

	if bytes.Compare(sum1, sum2) != 0 {
		t.Errorf(
			"Rollsums should not be different \"%v\" vs \"%v\"",
			sum1,
			sum2,
		)
	}
}

func TestThatSumDoesNotChangeTheHashState(t *testing.T) {
	r1 := NewRollsum16(4)

	sum1 := r1.Sum([]byte{1, 2, 3})
	sum2 := r1.Sum([]byte{3, 4, 5})

	if bytes.Compare(sum1[3:], sum2[3:]) != 0 {
		t.Errorf(
			"Rollsums should not be different \"%v\" vs \"%v\"",
			sum1,
			sum2,
		)
	}
}

func TestThatOutputLengthMatchesSize(t *testing.T) {
	r1 := NewRollsum16(4)
	sumLength := len(r1.Sum(nil))

	if sumLength != r1.Size() {
		t.Errorf("Unexpected length: %v vs expected %v", sumLength, r1.Size())
	}
}

func BenchmarkRollsum(b *testing.B) {
	r := NewRollsum16(100)
	buffer := make([]byte, 100)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))
	checksum := make([]byte, 16)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.Write(buffer)
		r.Sum(checksum)
		checksum = checksum[:0]
	}
	b.StopTimer()
}

func BenchmarkRollsum8096(b *testing.B) {
	r := NewRollsum16(8096)
	buffer := make([]byte, 8096)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))
	checksum := make([]byte, 16)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.Write(buffer)
		r.Sum(checksum)
		checksum = checksum[:0]
	}
	b.StopTimer()
}

func BenchmarkRollsum16Base(b *testing.B) {
	r := Rollsum16Base{blockSize: 100}
	buffer := make([]byte, 100)
	checksum := make([]byte, 16)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.SetBlock(buffer)
		r.GetSum(checksum)
	}
	b.StopTimer()

}

func BenchmarkMD5(b *testing.B) {
	hash := md5.New()
	buffer := make([]byte, 100)
	checksum := make([]byte, 32)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		hash.Write(buffer)
		hash.Sum(checksum)
		hash.Reset()
		checksum = checksum[:0]
	}
	b.StopTimer()
}

func BenchmarkMD5WithoutClear(b *testing.B) {
	hash := md5.New()
	buffer := make([]byte, 100)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		hash.Write(buffer)
		hash.Sum(nil)
		hash.Reset()
	}
	b.StopTimer()
}

// This is the benchmark where Rollsum should beat a full MD5 for each blocksize
func BenchmarkIncrementalRollsum(b *testing.B) {
	r := NewRollsum16(100)
	buffer := make([]byte, 100)
	r.Write(buffer)
	b.SetBytes(1)

	b.ReportAllocs()
	checksum := make([]byte, 16)
	increment := make([]byte, 1)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.Write(increment)
		r.Sum(checksum)
		checksum = checksum[:0]
	}
	b.StopTimer()
}

// The C2 veersion should avoid all allocations in the main loop, and beat the pants off the
// other versions
func BenchmarkIncrementalRollsumWithC2(b *testing.B) {
	const BLOCK_SIZE = 100
	r := NewRollsum16Base(BLOCK_SIZE)
	buffer := make([]byte, BLOCK_SIZE)
	b.SetBytes(1)
	cbuffer := circularbuffer.MakeC2Buffer(BLOCK_SIZE)

	r.AddBytes(buffer)
	cbuffer.Write(buffer)

	b.ReportAllocs()
	checksum := make([]byte, 16)
	increment := make([]byte, 1)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.AddBytes(increment)
		cbuffer.Write(increment)
		r.RemoveBytes(cbuffer.Evicted())
		r.GetSum(checksum)
	}
	b.StopTimer()
}
