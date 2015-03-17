package filechecksum

import (
	"crypto/md5"
	"testing"
)

type SingleBlockSource []byte

func (d SingleBlockSource) Get(blockID int) []byte {
	m := md5.New()
	m.Write(d)
	return m.Sum(nil)
}

func TestBlockEqualsItself(t *testing.T) {
	data := []byte("fooooo")

	h := HashVerifier{
		Hash:                md5.New(),
		BlockSize:           uint(len(data)),
		BlockChecksumGetter: SingleBlockSource(data),
	}

	if !h.VerifyBlockRange(0, data) {
		t.Error("data did not verify")
	}
}

type FourByteBlockSource []byte

func (d FourByteBlockSource) Get(blockID int) []byte {
	m := md5.New()
	m.Write(d[blockID*4 : (blockID+1)*4])
	return m.Sum(nil)
}

func TestSplitBlocksEqualThemselves(t *testing.T) {
	data := []byte("foooBaar")

	h := HashVerifier{
		Hash:                md5.New(),
		BlockSize:           uint(4),
		BlockChecksumGetter: FourByteBlockSource(data),
	}

	if !h.VerifyBlockRange(0, data) {
		t.Error("data did not verify")
	}
}
