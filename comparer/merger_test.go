package comparer

import (
	"testing"
)

func TestMergeAdjacentBlocksAfter(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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
	if merger.startEndBlockMap.Len() != 2 {
		t.Errorf("Wrong number of entries in the map: %v", merger.startEndBlockMap.Len())
	}
}

func TestMergeAdjacentBlocksBetween(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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
	if merger.startEndBlockMap.Len() != 2 {
		t.Errorf("Wrong number of entries in the map: %v", merger.startEndBlockMap.Len())
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
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

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

func TestBlockWithinSpan(t *testing.T) {
	// catch the case where we're informed about a block,
	// after we've merged blocks around it, so that the start and end
	// are within a span, not bordering one
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	merger.StartMergeResultStream(mergeChan, BLOCK_SIZE)

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 0,
		BlockIdx:         0,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: BLOCK_SIZE,
		BlockIdx:         1,
	}

	mergeChan <- BlockMatchResult{
		ComparisonOffset: 2 * BLOCK_SIZE,
		BlockIdx:         2,
	}

	// This one is a duplicate of an earlier one
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

	// start and end
	if merger.startEndBlockMap.Len() != 2 {
		t.Errorf("Wrong number of entries in the map: %v", merger.startEndBlockMap.Len())
	}
}

func TestNilBlockSpanList(t *testing.T) {
	s := BlockSpanList(nil)

	missing := s.GetMissingBlocks(1)

	if missing == nil {
		t.Fail()
	}

	if len(missing) == 0 {
		t.Fatal("missing should not be empty")
	}

	missingItem := missing[0]

	if missingItem.StartBlock != 0 {
		t.Errorf("Wrong startblock: %v", missingItem.StartBlock)
	}
	if missingItem.EndBlock != 1 {
		t.Errorf("Wrong endblock: %v", missingItem.EndBlock)
	}
}

func TestRegression1Merger(t *testing.T) {
	const BLOCK_SIZE = 4
	const ORIGINAL_STRING = "The quick brown fox jumped over the lazy dog"
	const MODIFIED_STRING = "The qwik brown fox jumped 0v3r the lazy"

	results, _ := compare(ORIGINAL_STRING, MODIFIED_STRING, BLOCK_SIZE)
	merger := &MatchMerger{}
	merger.StartMergeResultStream(results, BLOCK_SIZE)

	merged := merger.GetMergedBlocks()
	missing := merged.GetMissingBlocks(uint(len(ORIGINAL_STRING) / BLOCK_SIZE))

	expected := []string{
		"quic", "ed over ", " dog",
	}

	for i, v := range missing {
		start := v.StartBlock * BLOCK_SIZE
		end := (v.EndBlock + 1) * BLOCK_SIZE
		if end > uint(len(ORIGINAL_STRING)) {
			end = uint(len(ORIGINAL_STRING))
		}
		s := ORIGINAL_STRING[start:end]

		if s != expected[i] {
			t.Errorf("Wrong block %v: %v (expected %v)", i, expected[i])
		}
	}
}
