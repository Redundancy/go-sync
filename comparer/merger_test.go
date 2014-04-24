package comparer

import (
	"testing"
)

func TestMergeAdjacentBlocksAfter(t *testing.T) {
	const BLOCK_SIZE = 4

	mergeChan := make(chan BlockMatchResult)
	merger := &MatchMerger{}
	go merger.MergeResults(mergeChan, BLOCK_SIZE)

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
	go merger.MergeResults(mergeChan, BLOCK_SIZE)

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
	go merger.MergeResults(mergeChan, BLOCK_SIZE)

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
