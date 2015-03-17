package filechecksum

import (
	"bytes"
	"hash"
)

type ChecksumLookup interface {
	Get(blockID int) []byte
}

type HashVerifier struct {
	BlockSize           uint
	Hash                hash.Hash
	BlockChecksumGetter ChecksumLookup
}

func (v *HashVerifier) VerifyBlockRange(startBlockID uint, data []byte) bool {
	for i := 0; i*int(v.BlockSize) < len(data); i++ {
		blockData := data[i*int(v.BlockSize) : (i+1)*int(v.BlockSize)]

		expectedChecksum := v.BlockChecksumGetter.Get(int(startBlockID) + i)
		if expectedChecksum == nil {
			return true
		}

		v.Hash.Write(blockData)
		hashedData := v.Hash.Sum(nil)

		if bytes.Compare(expectedChecksum, hashedData) != 0 {
			return false
		}

		v.Hash.Reset()
	}

	return true
}
