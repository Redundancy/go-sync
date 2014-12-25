package rollsum

import (
	"bytes"
	"github.com/Redundancy/go-sync/circularbuffer"
	"hash"
	"io"
	"testing"
)

func TestThatRollsum32SatisfiesHashInterface(t *testing.T) {
	var i hash.Hash = NewRollsum32(10)
	i.Reset()
}

func TestThatRollsum32SatisfiedWriterInterface(t *testing.T) {
	var i io.Writer = NewRollsum32(10)
	n, err := i.Write([]byte{1, 2, 3, 4})

	if n != 4 {
		t.Error("Did not report writing 4 bytes")
	}

	if err != nil {
		t.Error(err)
	}
}

func TestThatRollsum32IsTheSameAfterBlockSizeBytes(t *testing.T) {
	r1 := NewRollsum32(4)
	r2 := NewRollsum32(4)

	r1.Write([]byte{1, 2, 3, 4})

	r2.Write([]byte{7, 6})
	r2.Write([]byte{5, 1, 2})
	r2.Write([]byte{3, 4})

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

func TestThatRollsum32IsTheSameAfterBlockSizeBytesWithPartialEviction(t *testing.T) {
	r1 := NewRollsum32(4)
	r2 := NewRollsum32(4)

	r1.Write([]byte{1, 2, 3, 4})

	r2.Write([]byte{7, 5})
	r2.Write([]byte{1, 2, 3, 4})

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

func TestRegression2(t *testing.T) {
	const A = "The quick br"
	const B = "The qwik br"

	r1 := NewRollsum32(4)
	r2 := NewRollsum32(4)

	r1.Write([]byte(A[:4]))
	r1.Reset()
	r1.Write([]byte(A[4:8]))
	r1.Reset()
	r1.Write([]byte(A[8:12]))

	r2.Write([]byte(B[:4]))
	r2.Write([]byte(B[4:8]))
	for _, c := range B[8:] {
		r2.Write([]byte{byte(c)})
	}

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

func TestThatRollsum32RemovesBytesCorrectly(t *testing.T) {
	r1 := NewRollsum32Base(2)

	r1.AddByte(255)
	r1.AddByte(10)
	r1.RemoveByte(255, 2)
	r1.AddByte(0)
	r1.RemoveByte(10, 2)
	r1.AddByte(0)

	if r1.a != 0 || r1.b != 0 {
		t.Errorf("Values are not reset: %v %v", r1.a, r1.b)
	}
}

func TestThatRollsum32IsDifferentForDifferentInput(t *testing.T) {
	r1 := NewRollsum32(4)
	r2 := NewRollsum32(4)

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

func TestResettingTheRollsum32(t *testing.T) {
	r1 := NewRollsum32(4)
	r2 := NewRollsum32(4)

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

func TestTruncatingPartiallyFilledBufferResultsInSameState(t *testing.T) {
	r1 := NewRollsum32Base(4)
	r2 := NewRollsum32Base(4)

	r1.AddByte(2)
	sum1 := make([]byte, 4)
	r1.GetSum(sum1)

	r2.AddByte(1)
	r2.AddByte(2)
	// Removal works from the left
	r2.RemoveByte(1, 2)
	sum2 := make([]byte, 4)
	r2.GetSum(sum2)

	if bytes.Compare(sum1, sum2) != 0 {
		t.Errorf(
			"Rollsums should not be different \"%v\" vs \"%v\"",
			sum1,
			sum2,
		)
	}
}

func TestThat32SumDoesNotChangeTheHashState(t *testing.T) {
	r1 := NewRollsum32(4)

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

func TestThat32OutputLengthMatchesSize(t *testing.T) {
	r1 := NewRollsum32(4)
	sumLength := len(r1.Sum(nil))

	if sumLength != r1.Size() {
		t.Errorf("Unexpected length: %v vs expected %v", sumLength, r1.Size())
	}
}

func BenchmarkRollsum32(b *testing.B) {
	r := NewRollsum32(100)
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

func BenchmarkRollsum32_8096(b *testing.B) {
	r := NewRollsum32(8096)
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

func BenchmarkRollsum32Base(b *testing.B) {
	r := Rollsum32Base{blockSize: 100}
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

// This is the benchmark where Rollsum should beat a full MD5 for each blocksize
func BenchmarkIncrementalRollsum32(b *testing.B) {
	r := NewRollsum32(100)
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
func BenchmarkIncrementalRollsum32WithC2(b *testing.B) {
	const BLOCK_SIZE = 100
	r := NewRollsum32Base(BLOCK_SIZE)
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
		cbuffer.Write(increment)
		r.AddAndRemoveBytes(increment, cbuffer.Evicted(), BLOCK_SIZE)
		r.GetSum(checksum)
	}
	b.StopTimer()
}
