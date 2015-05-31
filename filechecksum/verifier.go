package filechecksum

import (
	"bytes"
	"hash"
)

type ChecksumLookup interface {
	GetStrongChecksumForBlock(blockID int) []byte
}

type HashVerifier struct {
	BlockSize           uint
	Hash                hash.Hash
	BlockChecksumGetter ChecksumLookup
}

func (v *HashVerifier) VerifyBlockRange(startBlockID uint, data []byte) bool {
	for i := 0; i*int(v.BlockSize) < len(data); i++ {
		start := i * int(v.BlockSize)
		end := start + int(v.BlockSize)

		if end > len(data) {
			end = len(data)
		}

		blockData := data[start:end]

		expectedChecksum := v.BlockChecksumGetter.GetStrongChecksumForBlock(
			int(startBlockID) + i,
		)

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
