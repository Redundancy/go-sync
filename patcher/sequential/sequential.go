/*
Sequential Patcher will stream the patched version of the file to output,
since it works strictly in order, it cannot patch the local file directly
(since it might overwrite a block needed later),
so there would have to be a final copy once the patching was done.
*/
package sequential

import (
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"io"
)

const (
	ABSOLUTE_POSITION = 0
)

/*
This simple example currently doesn't do any pipelining of needed blocks, nor does it deal with
blocks being delivered out of order.
*/
func SequentialPatcher(
	localFile io.ReadSeeker,
	reference patcher.BlockSource,
	requiredRemoteBlocks []patcher.MissingBlockSpan,
	locallyAvailableBlocks []patcher.FoundBlockSpan,
	maxBlockStorage uint64, // the amount of memory we're allowed to use for temporary data storage
	output io.Writer,
) error {

	maxBlockMissing := uint(0)
	if len(requiredRemoteBlocks) > 0 {
		maxBlockMissing = requiredRemoteBlocks[len(requiredRemoteBlocks)-1].EndBlock
	}

	maxBlockFound := uint(0)
	if len(locallyAvailableBlocks) > 0 {
		maxBlockFound = locallyAvailableBlocks[len(locallyAvailableBlocks)-1].EndBlock
	}

	if reference == nil {
		return fmt.Errorf("No BlockSource set for obtaining reference blocks")
	}

	maxBlock := maxBlockMissing
	if maxBlockFound > maxBlock {
		maxBlock = maxBlockFound
	}

	currentBlock := uint(0)

	// TODO: find a way to test this, since it seemed to be the cause of an issue
	for currentBlock <= maxBlock {
		// where is the next block supposed to come from?
		if withinFirstBlockOfLocalBlocks(currentBlock, locallyAvailableBlocks) {
			firstMatched := locallyAvailableBlocks[0]

			// we have the current block range in the local file
			localFile.Seek(firstMatched.MatchOffset, ABSOLUTE_POSITION)
			blockSizeToRead := int64(firstMatched.EndBlock-firstMatched.StartBlock+1) * firstMatched.BlockSize

			if _, err := io.Copy(output, io.LimitReader(localFile, blockSizeToRead)); err != nil {
				return fmt.Errorf("Could not copy %v bytes to output: %v", blockSizeToRead, err)
			}

			currentBlock = firstMatched.EndBlock + 1
			locallyAvailableBlocks = locallyAvailableBlocks[1:]

		} else if withinFirstBlockOfRemoteBlocks(currentBlock, requiredRemoteBlocks) {
			firstMissing := requiredRemoteBlocks[0]
			reference.RequestBlocks(firstMissing)

			select {
			case result := <-reference.GetResultChannel():
				if result.StartBlock == currentBlock {
					if _, err := output.Write(result.Data); err != nil {
						return fmt.Errorf(
							"Could not write data to output: %v", 
							err,
						)
					} else {

						completed := calculateNumberOfCompletedBlocks(
							len(result.Data),
							firstMissing.BlockSize,
						)

						if completed != (firstMissing.EndBlock-firstMissing.StartBlock) + 1 {
							return fmt.Errorf(
								"Unexpected reponse length from remote source: blocks %v-%v (got %v blocks)", 
								firstMissing.StartBlock,
								firstMissing.EndBlock,
								completed,
						)
						}

						currentBlock += completed
						requiredRemoteBlocks = requiredRemoteBlocks[1:]
					}
				} else {
					return fmt.Errorf(
						"Received unexpected block: %v", 
						result.StartBlock,
					)
				}
			case err := <-reference.EncounteredError():
				return fmt.Errorf(
					"Failed to read from reference file: %v", 
					err,
				)
			}

		} else {
			return fmt.Errorf(
				"Could not find block in missing or matched list: %v\nRemote: %v\nLocal: %v\n",
				currentBlock,
				requiredRemoteBlocks,
				locallyAvailableBlocks,
			)
		}
	}

	return nil
}

func withinFirstBlockOfRemoteBlocks(currentBlock uint, remoteBlocks []patcher.MissingBlockSpan) bool {
	return len(remoteBlocks) > 0 && remoteBlocks[0].StartBlock <= currentBlock && remoteBlocks[0].EndBlock >= currentBlock
}

func withinFirstBlockOfLocalBlocks(currentBlock uint, localBlocks []patcher.FoundBlockSpan) bool {
	return len(localBlocks) > 0 && localBlocks[0].StartBlock <= currentBlock && localBlocks[0].EndBlock >= currentBlock
}

func calculateNumberOfCompletedBlocks(resultLength int, blockSize int64) (completedBlockCount uint) {
	// TODO: lots of casting to uint here, is it safe?
	completedBlockCount = uint(resultLength) / uint(blockSize)

	// round up in the case of a partial block (last block may not be full sized)
	if uint(resultLength)%uint(blockSize) != 0 {
		completedBlockCount += 1
	}

	return
}
