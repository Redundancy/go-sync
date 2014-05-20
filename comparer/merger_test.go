package comparer

import (
	"github.com/petar/GoLLRB/llrb"
	"testing"
)

func TestMergeAdjacentBlocksAfter(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         1,
	}

	close(mergeChan)

	merged := merger.GetMergedBlocks()

	if len(merged) != 1 {
		t.Fatalf("Wrong number of blocks returned: %#v", merged)
	}

	if merged[0].EndBlock != 1 {
		t.Errorf("Wrong EndBlock, expected 1 got %#v", merged[0])
	}
}

func TestMergeAdjacentBlocksBefore(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         1,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	close(mergeChan)

	merged := merger.GetMergedBlocks()

	if len(merged) != 1 {
		t.Fatalf("Wrong number of blocks returned: %#v", merged)
	}

	if merged[0].EndBlock != 1 {
		t.Errorf("Wrong EndBlock, expected 1 got %#v", merged[0])
	}

	// start and end
	if len(merger.startEndBlockMap) != 2 {
		t.Errorf("Wrong number of entries in the map: %v", len(merger.startEndBlockMap))
	}
}

func TestMergeAdjacentBlocksBetween(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 2 * BLOCK_SIZE,
		BlockIdx:         2,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	// match in the center
	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         1,
	}

	close(mergeChan)

	merged := merger.GetMergedBlocks()

	if len(merged) != 1 {
		t.Fatalf("Wrong number of blocks returned: %#v", merged)
	}

	if merged[0].EndBlock != 2 {
		t.Errorf("Wrong EndBlock, expected 2 got %#v", merged[0])
	}
	if merged[0].StartBlock != 0 {
		t.Errorf("Wrong StartBlock, expected 0, got %#v", merged[0])
	}
	if len(merger.startEndBlockMap) != 2 {
		t.Errorf("Wrong number of entries in the map: %v", len(merger.startEndBlockMap))
	}
}

func TestMissingBlocksOffsetStart(t *testing.T) {
	b := BlockSpanList{
		{
			StartBlock: 2,
			EndBlock:   3,
		},
	}

	m := b.GetMissingBlocks(3)

	if len(m) != 1 {
		t.Fatalf("Wrong number of missing blocks: %v", len(m))
	}

	if m[0].StartBlock != 0 {
		t.Errorf("Missing block has wrong start: %v", m[0].StartBlock)
	}
	if m[0].EndBlock != 1 {
		t.Errorf("Missing block has wrong end: %v", m[0].EndBlock)
	}
}

func TestMissingCenterBlock(t *testing.T) {
	b := BlockSpanList{
		{
			StartBlock: 0,
			EndBlock:   0,
		},
		{
			StartBlock: 2,
			EndBlock:   3,
		},
	}

	m := b.GetMissingBlocks(3)

	if len(m) != 1 {
		t.Fatalf("Wrong number of missing blocks: %v", len(m))
	}

	if m[0].StartBlock != 1 {
		t.Errorf("Missing block has wrong start: %v", m[0].StartBlock)
	}
	if m[0].EndBlock != 1 {
		t.Errorf("Missing block has wrong end: %v", m[0].EndBlock)
	}
}

func TestMissingEndBlock(t *testing.T) {
	b := BlockSpanList{
		{
			StartBlock: 0,
			EndBlock:   1,
		},
	}

	m := b.GetMissingBlocks(3)

	if len(m) != 1 {
		t.Fatalf("Wrong number of missing blocks: %v", len(m))
	}

	if m[0].StartBlock != 2 {
		t.Errorf("Missing block has wrong start: %v", m[0].StartBlock)
	}
	if m[0].EndBlock != 3 {
		t.Errorf("Missing block has wrong end: %v", m[0].EndBlock)
	}
}

func TestDuplicatedReferenceBlocks(t *testing.T) {
	// Reference = AA
	// Local = A
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	// When we find multiple strong matches, we send each of them
	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         1,
	}

	close(mergeChan)

	merged := merger.GetMergedBlocks()

	if len(merged) != 2 {
		t.Errorf("Duplicated blocks cannot be merged: %#v", merged)
	}

	missing := merged.GetMissingBlocks(1)

	if len(missing) > 0 {
		t.Errorf("There were no missing blocks: %#v", missing)
	}
}

func TestDuplicatedLocalBlocks(t *testing.T) {
	// Reference = A
	// Local = AA
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	// When we find multiple strong matches, we send each of them
	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         0,
	}

	close(mergeChan)

	// We only need one of the matches in the resulting file
	merged := merger.GetMergedBlocks()

	if len(merged) != 1 {
		t.Errorf("Duplicated blocks cannot be merged: %#v", merged)
	}

	missing := merged.GetMissingBlocks(0)

	if len(missing) > 0 {
		t.Errorf("There were no missing blocks: %#v", missing)
	}
}

func TestDoublyDuplicatedBlocks(t *testing.T) {
	// Reference = AA
	// Local = AA
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.MergeResults(mergeChan, BLOCK_SIZE)

	// When we find multiple strong matches, we send each of them
	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         1,
	}

	// Second local match
	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         1,
	}
	close(mergeChan)

	merged := merger.GetMergedBlocks()

	if len(merged) != 2 {
		t.Errorf("Duplicated blocks cannot be merged: %#v", merged)
	}

	missing := merged.GetMissingBlocks(1)

	if len(missing) > 0 {
		t.Errorf("There were no missing blocks: %#v", missing)
	}
}

// Just to test out usage of the LLRB interface and helpers
func TestLLRB(t *testing.T) {
	m := &MatchMerger{}
	m.startEndBlockMap2 = llrb.New()

	bm := m.startEndBlockMap2

	bm.ReplaceOrInsert(
		BlockSpanStart(
			BlockSpan{
				StartBlock: 0,
				EndBlock:   10,
			},
		),
	)

	bm.ReplaceOrInsert(
		BlockSpanEnd(
			BlockSpan{
				StartBlock: 0,
				EndBlock:   10,
			},
		),
	)

	i := bm.Get(BlockSpanKey(10))

	var EndBlock uint
	switch j := i.(type) {
	case BlockSpanStart:
		EndBlock = j.EndBlock
	case BlockSpanEnd:
		EndBlock = j.EndBlock
	}

	if EndBlock != 10 {
		t.Fail()
	}
}
