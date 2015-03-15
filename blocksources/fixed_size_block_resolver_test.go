package blocksources

import (
	"testing"
)

func TestNullResolverGivesBackTheSameBlocks(t *testing.T) {
	n := MakeNullFixedSizeResolver(5)
	result := n.SplitBlockRangeToDesiredSize(0, 10000)

	if len(result) != 1 {
		t.Fatalf("Unexpected result length (expected 1): %v", result)
	}

	r := result[0]

	if r.StartBlockID != 0 {
		t.Errorf("Unexpected start block ID: %v", r)
	}

	if r.EndBlockID != 10000 {
		t.Errorf("Unexpected end block ID: %v", r)
	}
}

func TestFixedSizeResolverSplitsBlocksOfDesiredSize(t *testing.T) {
	res := &FixedSizeBlockResolver{
		BlockSize:             5,
		MaxDesiredRequestSize: 5,
		FileSize:              20000,
	}

	// Should split two blocks, each of the desired request size
	// into two requests
	result := res.SplitBlockRangeToDesiredSize(0, 1)

	if len(result) != 2 {
		t.Fatalf("Unexpected result length (expected 2): %v", result)
	}

	if result[0].StartBlockID != 0 {
		t.Errorf("Unexpected start blockID: %v", result[0])
	}
	if result[0].EndBlockID != 0 {
		t.Errorf("Unexpected end blockID: %v", result[0])
	}

	if result[1].StartBlockID != 1 {
		t.Errorf("Unexpected start blockID: %v", result[1])
	}
	if result[1].EndBlockID != 1 {
		t.Errorf("Unexpected end blockID: %v", result[1])
	}
}

func TestThatMultipleBlocksAreSplitByRoundingDown(t *testing.T) {
	res := &FixedSizeBlockResolver{
		BlockSize:             5,
		MaxDesiredRequestSize: 12,
		FileSize:              20000,
	}

	// 0,1 (10) - 2-3 (10)
	result := res.SplitBlockRangeToDesiredSize(0, 3)

	if len(result) != 2 {
		t.Fatalf("Unexpected result length (expected 2): %v", result)
	}

	if result[0].StartBlockID != 0 {
		t.Errorf("Unexpected start blockID: %v", result[0])
	}
	if result[0].EndBlockID != 1 {
		t.Errorf("Unexpected end blockID: %v", result[0])
	}

	if result[1].StartBlockID != 2 {
		t.Errorf("Unexpected start blockID: %v", result[1])
	}
	if result[1].EndBlockID != 3 {
		t.Errorf("Unexpected end blockID: %v", result[1])
	}
}

func TestThatADesiredSizeSmallerThanABlockResultsInSingleBlocks(t *testing.T) {
	res := &FixedSizeBlockResolver{
		BlockSize:             5,
		MaxDesiredRequestSize: 4,
		FileSize:              20000,
	}

	// Should split two blocks
	result := res.SplitBlockRangeToDesiredSize(0, 1)

	if len(result) != 2 {
		t.Fatalf("Unexpected result length (expected 2): %v", result)
	}

	if result[0].StartBlockID != 0 {
		t.Errorf("Unexpected start blockID: %v", result[0])
	}
	if result[0].EndBlockID != 0 {
		t.Errorf("Unexpected end blockID: %v", result[0])
	}

	if result[1].StartBlockID != 1 {
		t.Errorf("Unexpected start blockID: %v", result[1])
	}
	if result[1].EndBlockID != 1 {
		t.Errorf("Unexpected end blockID: %v", result[1])
	}
}

func TestThatFileSizeTruncatesBlockEnds(t *testing.T) {
	res := &FixedSizeBlockResolver{
		BlockSize:             5,
		MaxDesiredRequestSize: 100,
		FileSize:              13,
	}

	// Should split two blocks
	result := res.GetBlockEndOffset(3)

	if result != 13 {
		t.Errorf("Unexpected BlockEnd Offset:", result)
	}
}
