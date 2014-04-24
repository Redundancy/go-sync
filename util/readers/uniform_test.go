package readers

import (
	"io"
	"io/ioutil"
	"testing"
)

func TestUniformReaderLength(t *testing.T) {
	r, err := ioutil.ReadAll(OneReader(100))

	if err != nil {
		t.Fatal(err)
	}

	if len(r) != 100 {
		t.Errorf("Unexpected length: %v", len(r))
	}

	for i, b := range r {
		if b != 1 {
			t.Errorf("Byte at position %v is not 1: %v", i, b)
		}
	}
}

func TestReadIntoLargerBuffer(t *testing.T) {
	b := make([]byte, 100)
	r := OneReader(10)

	n, err := r.Read(b)

	if n != 10 {
		t.Errorf("Wrong read length: %v", n)
	}

	if err != io.EOF {
		t.Errorf("Did not raise EOF after reading: %v", err)
	}
}

func TestMultiUniformReader(t *testing.T) {
	r := io.MultiReader(
		OneReader(12),
		NewSizedNonRepeatingSequence(0, 88),
	)

	b := make([]byte, 100)

	n, err := r.Read(b)

	if n != 12 {
		t.Errorf("Wrong read length: %v", n)
	}

	if err == io.EOF {
		t.Errorf("Raised EOF after reading! %v", err)
	}

	n, err = r.Read(b)

	if n != 88 {
		t.Errorf("Wrong read length: %v", n)
	}

	n, err = r.Read(b)

	if err != io.EOF {
		t.Errorf("Really expected EOF by now: %v %v", err, n)
	}
}

func TestFillBuffer(t *testing.T) {
	r := io.MultiReader(
		OneReader(12),
		NewSizedNonRepeatingSequence(0, 88),
	)

	b := make([]byte, 100)
	_, err := io.ReadFull(r, b)

	if err != nil && err != io.EOF {
		t.Error(err)
	}

	if len(b) != cap(b) {
		t.Errorf("Expected to fill b: %v", len(b))
	}

}
