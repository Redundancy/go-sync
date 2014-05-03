/*
Sequential Patcher will stream the patched version of the file to output,
since it works strictly in order, it cannot patch the local file directly,
so there would have to be a final copy once the patching was done.
*/
package sequential

import (
	"fmt"
	"github.com/Redundancy/go-sync/patcher"
	"io"
)

/*
This simple example currently doesn't do any pipelining of needed blocks, nor does it deal with
blocks being delivered out of order.
*/
func SequentialPatcher(
	localFile io.ReadSeeker,
	reference patcher.BlockSource,
	missingLocal []patcher.MissingBlockSpan,
	matchedLocal []patcher.FoundBlockSpan,
	maxBlockStorage uint64, // the amount of memory we're allowed to use for temporary data storage
	output io.Writer,
) error {
	maxBlockMissing := uint(0)
	if len(missingLocal) > 0 {
		maxBlockMissing = missingLocal[len(missingLocal)-1].EndBlock
	}

	maxBlockFound := uint(0)
	if len(matchedLocal) > 0 {
		maxBlockFound = matchedLocal[len(matchedLocal)-1].EndBlock
	}

	if reference == nil {
		return fmt.Errorf("No BlockSource set for obtaining reference blocks")
	}

	maxBlock := maxBlockMissing
	if maxBlockFound > maxBlock {
		maxBlock = maxBlockFound
	}

	currentBlock := uint(0)
	missing := missingLocal
	matched := matchedLocal

	for currentBlock < maxBlock {
		// where is the next block supposed to come from?
		if len(matched) > 0 && matched[0].StartBlock <= currentBlock && matched[0].EndBlock >= currentBlock {
			firstMatched := matched[0]

			// we have the current block range in the local file
			localFile.Seek(firstMatched.MatchOffset, 0)
			blockSizeToRead := int64(firstMatched.EndBlock-firstMatched.StartBlock+1) * firstMatched.BlockSize

			if _, err := io.Copy(output, io.LimitReader(localFile, blockSizeToRead)); err != nil {
				return err
			}

			currentBlock = firstMatched.EndBlock + 1
			matched = matched[1:]

		} else if len(missing) > 0 && missing[0].StartBlock <= currentBlock && missing[0].EndBlock >= currentBlock {
			firstMissing := missing[0]
			reference.RequestBlock(firstMissing)

			select {
			case result := <-reference.GetResultChannel():
				if result.StartBlock == currentBlock {
					if _, err := output.Write(result.Data); err != nil {
						return err
					} else {
						advance := uint(int64(len(result.Data)) / firstMissing.BlockSize)
						if int64(len(result.Data))%firstMissing.BlockSize != 0 {
							advance += 1
						}
						currentBlock += advance
						missing = missing[1:]
					}
				} else {
					return fmt.Errorf("Received unexpected block: %v", result.StartBlock)
				}
			case err := <-reference.EncounteredError():
				return err
			}

		} else {
			return fmt.Errorf("Could not find block in missing or matched list: %v - %v %v", currentBlock, missing, matched)
		}
	}

	return nil
}
