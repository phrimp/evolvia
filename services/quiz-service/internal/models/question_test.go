package models

import (
	"testing"
)

func TestBloomScoreCalculation(t *testing.T) {
	testCases := []struct {
		bloomLevel      string
		expectedScore   int
		expectEasyScore int
		expectHardScore int
	}{
		{"remember", 10, 10, 15},
		{"understand", 15, 15, 22},
		{"apply", 20, 20, 30},
		{"analyze", 25, 25, 37},
		{"evaluate", 30, 30, 45},
		{"create", 35, 35, 52},
		{"invalid", 10, 10, 15}, // fallback to default
	}

	for _, tc := range testCases {
		t.Run(tc.bloomLevel, func(t *testing.T) {
			question := &Question{
				BloomLevel: tc.bloomLevel,
			}

			// Test base score calculation
			question.CalculateBloomScore()
			if question.BloomScore != tc.expectedScore {
				t.Errorf("Expected BloomScore %d, got %d", tc.expectedScore, question.BloomScore)
			}

			// Test stage score calculation
			question.CalculateBloomScoresByStage()

			if len(question.BloomScoresByStage) != 3 {
				t.Errorf("Expected 3 stage scores, got %d", len(question.BloomScoresByStage))
			}

			if question.BloomScoresByStage["easy"] != tc.expectEasyScore {
				t.Errorf("Expected easy score %d, got %d", tc.expectEasyScore, question.BloomScoresByStage["easy"])
			}

			if question.BloomScoresByStage["hard"] != tc.expectHardScore {
				t.Errorf("Expected hard score %d, got %d", tc.expectHardScore, question.BloomScoresByStage["hard"])
			}

			// Test GetScoreForStage method
			easyScore := question.GetScoreForStage("easy")
			if easyScore != tc.expectEasyScore {
				t.Errorf("GetScoreForStage('easy') expected %d, got %d", tc.expectEasyScore, easyScore)
			}

			hardScore := question.GetScoreForStage("hard")
			if hardScore != tc.expectHardScore {
				t.Errorf("GetScoreForStage('hard') expected %d, got %d", tc.expectHardScore, hardScore)
			}
		})
	}
}

func TestEnsureBloomScores(t *testing.T) {
	question := &Question{
		BloomLevel: "analyze",
	}

	// Initially should be zero
	if question.BloomScore != 0 {
		t.Errorf("Expected initial BloomScore to be 0, got %d", question.BloomScore)
	}

	// Call EnsureBloomScores
	question.EnsureBloomScores()

	// Should now be calculated
	if question.BloomScore != 25 {
		t.Errorf("Expected BloomScore to be 25 after EnsureBloomScores, got %d", question.BloomScore)
	}

	if len(question.BloomScoresByStage) != 3 {
		t.Errorf("Expected 3 stage scores after EnsureBloomScores, got %d", len(question.BloomScoresByStage))
	}
}
