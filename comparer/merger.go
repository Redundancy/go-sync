package comparer

import (
	"sort"
	"sync"

	"github.com/petar/GoLLRB/llrb"
)

// The result merger takes many BlockMatchResult
// and combines adjoining results into spans of blocks
type MatchMerger struct {
	sync.Mutex

	wait sync.WaitGroup

	// Porting from map to llrb, to enable finding of the next
	// largest blockID (What's the first blockID after 5?)
	startEndBlockMap *llrb.LLRB

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

func itemToBlockSpan(in llrb.Item) BlockSpan {
	switch i := in.(type) {
	case BlockSpanStart:
		return BlockSpan(i)
	case BlockSpanEnd:
		return BlockSpan(i)
	}
	return BlockSpan{}
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
		merger.startEndBlockMap.Delete(BlockSpanKey(a.EndBlock))
		merger.startEndBlockMap.Delete(BlockSpanKey(b.StartBlock))
		a.EndBlock = b.EndBlock

		merger.startEndBlockMap.ReplaceOrInsert(BlockSpanStart(*a))
		merger.startEndBlockMap.ReplaceOrInsert(BlockSpanEnd(*a))
	}
}

// Can be used on multiple streams of results simultaneously
// starts working asyncronously, call from the initiating goroutine
func (merger *MatchMerger) StartMergeResultStream(
	resultStream <-chan BlockMatchResult,
	blockSize int64,
) {
	// Add should be called on the main goroutine
	// to ensure that it has happened before wait is called
	merger.wait.Add(1)

	if merger.startEndBlockMap == nil {
		merger.startEndBlockMap = llrb.New()
	}

	// used by the llrb iterator to signal that it found/didn't find
	// an existing key on or spanning the given block
	//foundExisting := make(chan bool)

	go func() {
		defer merger.wait.Done()

		for result := range resultStream {
			if result.Err != nil {
				return
			}

			merger.Lock()
			merger.blockCount += 1

			blockID := result.BlockIdx
			preceeding := merger.startEndBlockMap.Get(BlockSpanKey(blockID - 1))
			following := merger.startEndBlockMap.Get(BlockSpanKey(blockID + 1))

			asBlockSpan := toBlockSpan(result)

			var foundExisting bool
			// Exists, or within an existing span
			merger.startEndBlockMap.AscendGreaterOrEqual(
				BlockSpanKey(blockID),
				// iterator
				func(i llrb.Item) bool {
					j, ok := i.(BlockSpanIndex)

					if !ok {
						foundExisting = true
						return false
					}

					switch k := j.(type) {
					case BlockSpanStart:
						// it's only overlapping if its the same blockID
						foundExisting = k.StartBlock == blockID
						return false

					case BlockSpanEnd:
						// we didn't find a start, so there's an end that overlaps
						foundExisting = true
						return false
					default:
						foundExisting = true
						return false
					}

				},
			)

			if foundExisting {
				merger.Unlock()
				continue
			}

			merger.startEndBlockMap.ReplaceOrInsert(
				BlockSpanStart(*toBlockSpan(result)),
			)

			if preceeding != nil && following != nil {
				a := itemToBlockSpan(preceeding)
				merger.merge(
					asBlockSpan,
					&a,
					blockSize,
				)

				b := itemToBlockSpan(following)
				merger.merge(
					&a,
					&b,
					blockSize,
				)

			} else if preceeding != nil {
				a := itemToBlockSpan(preceeding)
				merger.merge(
					asBlockSpan,
					&a,
					blockSize,
				)
			} else if following != nil {
				b := itemToBlockSpan(following)
				merger.merge(
					asBlockSpan,
					&b,
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
	m := merger.startEndBlockMap

	m.AscendGreaterOrEqual(m.Min(), func(item llrb.Item) bool {
		switch block := item.(type) {
		case BlockSpanStart:
			sorted = append(sorted, BlockSpan(block))
			smallestKey = block.StartBlock + 1
		}
		return true
	})

	sort.Sort(sorted)
	return
}

// Creates a list of spans that are missing.
// note that maxBlock is blockCount-1
func (l BlockSpanList) GetMissingBlocks(maxBlock uint) (sorted BlockSpanList) {
	// it's difficult to know how many spans we will need
	sorted = make(BlockSpanList, 0)

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

	if lastBlockSpanIndex == -1 {
		sorted = append(
			sorted,
			BlockSpan{
				StartBlock: 0,
				EndBlock:   maxBlock,
			},
		)
	} else if uint(lastBlockSpanIndex) < maxBlock {
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
