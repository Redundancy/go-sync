package circularbuffer

import (
	"testing"
)

func TestMakeCircularBuffer(t *testing.T) {
	b := NewCircularBuffer(100)

	if b == nil {
		t.Error("No Circular buffer created")
	}
}

func TestWriteAndGet(t *testing.T) {
	b := NewCircularBuffer(100)
	const TEST = "abcd"

	_, err := b.Write([]byte(TEST))

	if err != nil {
		t.Fatal(err)
	}

	got := string(b.Get())

	if got != TEST {
		t.Errorf("Got unexpected buffer content: %v", got)
	}

}

func TestWriteEvicedEmpty(t *testing.T) {
	b := NewCircularBuffer(100)
	const TEST = "abcd"
	overWrote := b.WriteEvicted([]byte(TEST))

	if overWrote != nil {
		t.Errorf("Write on empty buffer should not evict anything: %s", overWrote)
	}
}

func TestWriteMoreThanFullBuffer(t *testing.T) {
	b := NewCircularBuffer(2)

	r1 := b.WriteEvicted([]byte("abcd"))

	if r1 != nil {
		t.Errorf("Did not expect an output: %s", r1)
	}

	r2 := b.WriteEvicted([]byte("ef"))

	if string(r2) != "cd" {
		t.Errorf("ef should have overwritten cd: %v", r2)
	}
}

func TestOverwritingChunk(t *testing.T) {
	b := NewCircularBuffer(4)
	// filled
	b.WriteEvicted([]byte("abcd"))

	// should overwrite ab
	b.WriteEvicted([]byte("ef"))

	// should overwrite cd
	overwritten := b.WriteEvicted([]byte("gh"))

	if string(overwritten) != "cd" {
		t.Errorf("circular buffer should have overwritten \"cd\", overwrite \"%v\"", string(overwritten))
	}
}

func TestGetWhenInMiddleOfBuffer(t *testing.T) {
	b := NewCircularBuffer(4)
	// filled
	b.WriteEvicted([]byte("abcd"))
	// should overwrite ab
	b.WriteEvicted([]byte("ef"))

	got := b.Get()

	if string(got) != "cdef" {
		t.Errorf("Unexpected buffer when part way through: %s", got)
	}
}
