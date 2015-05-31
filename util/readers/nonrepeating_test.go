package readers

import (
	"io"
	"io/ioutil"
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
		t.Fatalf("Bytes should not be the same! %s vs %s", a, b)
	}
}

func TestNonRepeatingSequenceIsDifferent(t *testing.T) {
	i := NewNonRepeatingSequence(0)
	i2 := NewNonRepeatingSequence(5)

	a := []byte{0}
	b := []byte{0}

	commonalities := 0

	for x := 0; x < 100; x++ {
		i.Read(a)
		i2.Read(b)

		if a[0] == b[0] {
			commonalities += 1
		}
	}

	if commonalities > 5 {
		t.Fatal("Sequences are too similar")
	}
}

func BenchmarkNonRepeatingSequence(b *testing.B) {
	b.SetBytes(1)

	s := NewSizedNonRepeatingSequence(0, int64(b.N))

	b.StartTimer()
	_, err := io.Copy(ioutil.Discard, s)
	b.StopTimer()

	if err != nil {
		b.Fatal(err)
	}
}
