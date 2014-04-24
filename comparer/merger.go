package comparer

import (
	"sort"
	"sync"
)

// The result merger takes many BlockMatchResult
// and combines adjoining results into spans of blocks
type MatchMerger struct {
	sync.Mutex

	wait sync.WaitGroup
	// BlockSpans are stored by both start and end block ids
	// if anything shares borders of these, they should be merged
	startEndBlockMap map[uint]*BlockSpan
}

// a span of multiple blocks, from start to end, which match the blocks
// starting at an offset of ComparisonStartOffset
type BlockSpan struct {
	StartBlock uint
	EndBlock   uint

	// byte offset in the comparison for the match
	ComparisonStartOffset int64
}

func (b BlockSpan) EndOffset(blockSize int64) int64 {
	return b.ComparisonStartOffset + blockSize*int64(b.EndBlock-b.StartBlock+1)
}

func toBlockSpan(b BlockMatchResult) *BlockSpan {
	return &BlockSpan{
		StartBlock:            b.BlockIdx,
		EndBlock:              b.BlockIdx,
		ComparisonStartOffset: b.ComparisonOffset,
	}
}

func isBordering(a, b *BlockSpan, blockSize int64) bool {
	if a.EndBlock == b.StartBlock-1 && a.EndOffset(blockSize) == b.ComparisonStartOffset {
		return true
	} else if b.EndBlock == a.StartBlock-1 && b.EndOffset(blockSize) == a.ComparisonStartOffset {
		return true
	}

	return false
}

// if merged, the block span remaining is the one with the lower start block
func (merger *MatchMerger) merge(block1, block2 *BlockSpan, blockSize int64) {
	var a, b *BlockSpan = block1, block2

	if block1.StartBlock > block2.StartBlock {
		a, b = block2, block1
	}

	if isBordering(a, b, blockSize) {
		// bordering, merge
		// A ------ A B ------ B > A ---------------- A
		delete(merger.startEndBlockMap, a.EndBlock)
		delete(merger.startEndBlockMap, b.StartBlock)
		a.EndBlock = b.EndBlock

		merger.startEndBlockMap[a.StartBlock] = a
		merger.startEndBlockMap[a.EndBlock] = a
	}
}

// Can be used on multiple streams of results simultaneously
func (merger *MatchMerger) MergeResults(
	resultStream <-chan BlockMatchResult,
	blockSize int64,
) {
	merger.wait.Add(1)
	defer merger.wait.Done()

	for result := range resultStream {
		if result.Err != nil {
			return
		}

		merger.Lock()
		if merger.startEndBlockMap == nil {
			merger.startEndBlockMap = make(map[uint]*BlockSpan)
		}

		blockID := result.BlockIdx
		preceeding, foundBefore := merger.startEndBlockMap[blockID-1]
		following, foundAfter := merger.startEndBlockMap[blockID+1]

		asBlockSpan := toBlockSpan(result)
		merger.startEndBlockMap[blockID] = toBlockSpan(result)

		if foundBefore && foundAfter {
			merger.merge(
				asBlockSpan,
				preceeding,
				blockSize,
			)

			merger.merge(
				preceeding,
				following,
				blockSize,
			)

		} else if foundBefore {
			merger.merge(
				asBlockSpan,
				preceeding,
				blockSize,
			)
		} else if foundAfter {
			merger.merge(
				asBlockSpan,
				following,
				blockSize,
			)
		}

		merger.Unlock()
	}
}

type BlockSpanList []BlockSpan

func (l BlockSpanList) Len() int {
	return len(l)
}

func (l BlockSpanList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l BlockSpanList) Less(i, j int) bool {
	return l[i].StartBlock < l[j].StartBlock
}

// Sorted list of blocks, based on StartBlock
func (merger *MatchMerger) GetMergedBlocks() (sorted BlockSpanList) {
	merger.wait.Wait()
	var smallestKey uint = 0

	for _, block := range merger.startEndBlockMap {
		if block.StartBlock >= smallestKey {
			sorted = append(sorted, *block)
			smallestKey = block.StartBlock + 1
		}
	}

	sort.Sort(sorted)
	return
}
