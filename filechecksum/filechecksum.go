/*
package filechecksum provides the FileChecksumGenerator, whose main responsibility is to read a file,
and generate both weak and strong checksums for every block. It is also used by the comparer, which
will generate weak checksums for potential byte ranges that could match the index, and strong checksums
if needed.
*/
package filechecksum

import (
	"crypto/md5"
	"github.com/Redundancy/go-sync/rollsum"
	"hash"
	"io"
)

// Rsync swapped to this after version 30
// this is a factory function, because we don't actually want to share hash state
var DefaultStrongHashGenerator = func() hash.Hash { return md5.New() }

// We provide an overall hash of individual files
var DefaultFileHashGenerator = func() hash.Hash { return md5.New() }

// Uses all default hashes (MD5 & rollsum16)
func NewFileChecksumGenerator(blocksize uint) *FileChecksumGenerator {
	return &FileChecksumGenerator{
		BlockSize:        blocksize,
		WeakRollingHash:  rollsum.NewRollsum16Base(blocksize),
		StrongHash:       DefaultStrongHashGenerator(),
		FileChecksumHash: DefaultFileHashGenerator(),
	}
}

type RollingHash interface {
	// the size of the hash output
	Size() int

	AddByte(b byte)
	RemoveByte(b byte)

	AddBytes(bs []byte)
	RemoveBytes(bs []byte)

	SetBlock(block []byte)

	GetSum(b []byte)
	Reset()
}

/*
FileChecksumGenerator provides a description of what hashing functions to use to
evaluate a file. Since the hashes store state, it is NOT safe to use a generator concurrently
for different things.
*/
type FileChecksumGenerator struct {
	// See BlockBuffer
	WeakRollingHash  RollingHash
	StrongHash       hash.Hash
	FileChecksumHash hash.Hash
	BlockSize        uint
}

// Reset all hashes to initial state
func (check *FileChecksumGenerator) Reset() {
	check.WeakRollingHash.Reset()
	check.StrongHash.Reset()
	check.FileChecksumHash.Reset()
}

func (check *FileChecksumGenerator) ChecksumSize() int {
	return check.WeakRollingHash.Size() + check.GetStrongHash().Size()
}

func (check *FileChecksumGenerator) GetChecksumSizes() (int, int) {
	return check.WeakRollingHash.Size(), check.GetStrongHash().Size()
}

// Gets the Hash function for the overall file used on each block
// defaults to md5
func (check *FileChecksumGenerator) GetFileHash() hash.Hash {
	return check.FileChecksumHash
}

// Gets the Hash function for the strong hash used on each block
// defaults to md5, but can be overriden by the generator
func (check *FileChecksumGenerator) GetStrongHash() hash.Hash {
	return check.StrongHash
}

// GenerateChecksums reads each block of the input file, and outputs first the weak, then the strong checksum
// to the output writer. It will return a checksum for the whole file.
// Potentially speaking, this might be better producing a channel of blocks, which would remove the need for io from
// a number of other places.
// TODO: Make this output a channel of blocks, move the block loading / writing logic out
func (check *FileChecksumGenerator) GenerateChecksums(inputFile io.Reader, output io.Writer) (fileChecksum []byte, err error) {
	fullChecksum := check.GetFileHash()
	strongHash := check.GetStrongHash()

	buffer := make([]byte, check.BlockSize)

	// ensure that the hashes are clean
	strongHash.Reset()
	fullChecksum.Reset()

	// We reset the hashes when done do we can reuse the generator
	defer check.WeakRollingHash.Reset()
	defer strongHash.Reset()
	defer fullChecksum.Reset()

	// ensure preallocated memory
	strongChecksumValue := make([]byte, 0, strongHash.Size())
	weakChecksumValue := make([]byte, check.WeakRollingHash.Size())

	for {
		n, err := io.ReadFull(inputFile, buffer)
		section := buffer[:n]

		if n == 0 {
			break
		}

		// As hashes, the assumption is that they never error
		// additionally, we assume that the only reason not
		// to write a full block would be reaching the end of the file
		fullChecksum.Write(section)
		check.WeakRollingHash.SetBlock(section)
		strongHash.Write(section)

		check.WeakRollingHash.GetSum(weakChecksumValue)
		output.Write(weakChecksumValue)

		strongChecksumValue = strongHash.Sum(strongChecksumValue)
		output.Write(strongChecksumValue)
		// clear it again
		strongChecksumValue = strongChecksumValue[:0]

		// Reset the strong
		strongHash.Reset()

		if n != len(buffer) || err == io.EOF {
			break
		}
	}

	return fullChecksum.Sum(nil), nil
}
