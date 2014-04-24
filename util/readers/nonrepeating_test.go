package readers

import (
	"testing"
)

// This is only a very basic test
func TestNonRepeatingSequenceReader(t *testing.T) {
	i := NewNonRepeatingSequence(0)
	a := []byte{0}
	b := []byte{0}

	i.Read(a)
	i.Read(b)

	if a[0] == b[0] {
		t.Fatal("Bytes should not be the same! %s vs %s", a, b)
	}
}
