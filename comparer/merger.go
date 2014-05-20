package comparer

import (
	"github.com/petar/GoLLRB/llrb"
	"sort"
	"sync"
)

// The result merger takes many BlockMatchResult
// and combines adjoining results into spans of blocks
type MatchMerger struct {
	sync.Mutex

	wait sync.WaitGroup

	// Porting from map to llrb, to enable finding of the next
	// largest blockID (What's the first blockID after 5?)
	startEndBlockMap2 *llrb.LLRB

	// BlockSpans are stored by both start and end block ids
	// if anything shares borders of these, they should be merged
	startEndBlockMap map[uint]*BlockSpan

	blockCount uint
}

// a span of multiple blocks, from start to end, which match the blocks
// starting at an offset of ComparisonStartOffset
type BlockSpan struct {
	StartBlock uint
	EndBlock   uint

	// byte offset in the comparison for the match
	ComparisonStartOffset int64
}

type BlockSpanIndex interface {
	Position() uint
}

// Wraps a blockspan so that it may easily be used
// in llrb. Corresponds to the start block of the blockspan.
type BlockSpanStart BlockSpan

func (s BlockSpanStart) Position() uint {
	return s.StartBlock
}

func (s BlockSpanStart) Less(than llrb.Item) bool {
	return s.StartBlock < than.(BlockSpanIndex).Position()
}

// Wraps a blockspan so that it may easily be used
// in llrb. Corresponds to the end of the blockspan.
type BlockSpanEnd BlockSpan

func (s BlockSpanEnd) Position() uint {
	return s.EndBlock
}

func (s BlockSpanEnd) Less(than llrb.Item) bool {
	return s.EndBlock < than.(BlockSpanIndex).Position()
}

// Wraps a block index, allowing easy use of llrb.Get()
type BlockSpanKey uint

func (s BlockSpanKey) Position() uint {
	return uint(s)
}

func (k BlockSpanKey) Less(than llrb.Item) bool {
	return uint(k) < than.(BlockSpanIndex).Position()
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
// starts working asyncronously, call from the initiating goroutine
func (merger *MatchMerger) MergeResults(
	resultStream <-chan BlockMatchResult,
	blockSize int64,
) {
	// Add should be called on the main goroutine
	// to ensure that it has happened before wait is called
	merger.wait.Add(1)

	go func() {
		defer merger.wait.Done()

		for result := range resultStream {
			if result.Err != nil {
				return
			}

			merger.Lock()
			merger.blockCount += 1

			if merger.startEndBlockMap == nil {
				merger.startEndBlockMap = make(map[uint]*BlockSpan)
			}

			blockID := result.BlockIdx
			preceeding, foundBefore := merger.startEndBlockMap[blockID-1]
			following, foundAfter := merger.startEndBlockMap[blockID+1]

			asBlockSpan := toBlockSpan(result)

			if _, blockAlreadyExists := merger.startEndBlockMap[blockID]; blockAlreadyExists {
				merger.Unlock()
				continue
			}

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
	}()
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

// Creates a list of spans that are missing.
func (l BlockSpanList) GetMissingBlocks(maxBlock uint) (sorted BlockSpanList) {
	// it's difficult to know how many spans we will need
	sorted = make(BlockSpanList, 0, maxBlock/4)

	lastBlockSpanIndex := -1
	for _, blockSpan := range l {
		if int(blockSpan.StartBlock) > lastBlockSpanIndex+1 {
			sorted = append(
				sorted,
				BlockSpan{
					StartBlock: uint(lastBlockSpanIndex + 1),
					EndBlock:   blockSpan.StartBlock - 1,
				},
			)
		}

		lastBlockSpanIndex = int(blockSpan.EndBlock)
	}

	if uint(lastBlockSpanIndex) < maxBlock {
		sorted = append(
			sorted,
			BlockSpan{
				StartBlock: uint(lastBlockSpanIndex + 1),
				EndBlock:   maxBlock,
			},
		)
	}

	return sorted
}
