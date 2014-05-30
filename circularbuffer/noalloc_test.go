package circularbuffer

import (
	"bytes"
	"testing"
)

const BLOCK_SIZE = 10

var incrementBlock = make([]byte, BLOCK_SIZE)
var incrementBlock2 = make([]byte, BLOCK_SIZE)

func init() {
	for i, _ := range incrementBlock {
		incrementBlock[i] = byte(i)
		incrementBlock2[i] = byte(i + BLOCK_SIZE)
	}
}

func TestCreateC2Buffer(t *testing.T) {
	MakeC2Buffer(BLOCK_SIZE)
}

func TestWriteBlock(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)
}

func TestWritingUnderCapacityGivesEmptyEvicted(t *testing.T) {
	b := MakeC2Buffer(2)
	b.Write([]byte{1, 2})

	if len(b.Evicted()) != 0 {
		t.Fatal("Evicted should have been empty:", b.Evicted())
	}
}

func TestWritingMultipleBytesWhenBufferIsNotFull(t *testing.T) {
	b := MakeC2Buffer(3)
	b.Write([]byte{1, 2})
	b.Write([]byte{3, 4})

	ev := b.Evicted()

	if len(ev) != 1 || ev[0] != 1 {
		t.Fatal("Evicted should have been [1,]:", ev)
	}
}

func TestEvictedRegession1(t *testing.T) {
	b := MakeC2Buffer(4)

	b.Write([]byte{7, 6})
	b.Write([]byte{5, 1, 2})
	b.Write([]byte{3, 4})

	ev := b.Evicted()
	if len(ev) != 2 || ev[0] != 6 || ev[1] != 5 {
		t.Fatalf("Unexpected evicted [6,5]: %v", ev)
	}
}

func TestGetBlock(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)

	block := b.GetBlock()

	if len(block) != BLOCK_SIZE {
		t.Fatal("Wrong block size returned")
	}

	for i, by := range block {
		if byte(i) != by {
			t.Errorf("byte %v does not match", i)
		}
	}
}

func TestWriteTwoBlocksGet(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)
	b.Write(incrementBlock2)

	if bytes.Compare(b.GetBlock(), incrementBlock2) != 0 {
		t.Errorf("Get block did not return the right value: %s", b.GetBlock())
	}
}

func TestWriteSingleByteGetSingleByte(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	singleByte := []byte{0}
	b.Write(singleByte)

	if bytes.Compare(b.GetBlock(), singleByte) != 0 {
		t.Errorf("Get block did not return the right value: %s", b.GetBlock())
	}
}

func TestWriteTwoBlocksGetEvicted(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)
	b.Write(incrementBlock2)

	if bytes.Compare(b.Evicted(), incrementBlock) != 0 {
		t.Errorf("Evicted did not return the right value: %s", b.Evicted())
	}
}

func TestWriteSingleByteReturnsSingleEvictedByte(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock2)
	singleByte := []byte{0}

	b.Write(singleByte)
	e := b.Evicted()

	if len(e) != 1 {
		t.Fatalf("Evicted length is not correct: %s", e)
	}

	if e[0] != byte(10) {
		t.Errorf("Evicted content is not correct: %s", e)
	}
}

func TestTruncatingAfterWriting(t *testing.T) {
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)

	evicted := b.Truncate(2)

	if len(evicted) != 2 {
		t.Fatalf("Truncate did not return expected evicted length: %v", evicted)
	}

	if evicted[0] != 0 || evicted[1] != 1 {
		t.Errorf("Unexpected content in evicted: %v", evicted)
	}
}

func TestWritingAfterTruncating(t *testing.T) {
	// test that after we truncate some content, the next operations
	// on the buffer give us the expected results
	b := MakeC2Buffer(BLOCK_SIZE)
	b.Write(incrementBlock)
	b.Truncate(4)

	b.Write([]byte{34, 46})

	block := b.GetBlock()

	if len(block) != BLOCK_SIZE-2 {
		t.Fatalf(
			"Unexpected block length after truncation: %v (%v)",
			block,
			len(block),
		)
	}

	if bytes.Compare(block, []byte{4, 5, 6, 7, 8, 9, 34, 46}) != 0 {
		t.Errorf(
			"Unexpected block content after truncation: %v (%v)",
			block,
			len(block))
	}
}

// This should have no allocations!
func BenchmarkSingleWrites(b *testing.B) {
	buffer := MakeC2Buffer(BLOCK_SIZE)
	buffer.Write(incrementBlock)
	b.ReportAllocs()

	singleByte := []byte{0}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buffer.Write(singleByte)
		buffer.Evicted()
	}
	b.StopTimer()
}
