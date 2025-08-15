package constants

// BloomTaxonomyLevels defines the standard Bloom's taxonomy levels in order
const (
	BloomLevelRemember   = "remember"
	BloomLevelUnderstand = "understand"
	BloomLevelApply      = "apply"
	BloomLevelAnalyze    = "analyze"
	BloomLevelEvaluate   = "evaluate"
	BloomLevelCreate     = "create"
)

// BloomTaxonomyLevels provides ordered list of all Bloom taxonomy levels
var BloomTaxonomyLevels = []string{
	BloomLevelRemember,
	BloomLevelUnderstand,
	BloomLevelApply,
	BloomLevelAnalyze,
	BloomLevelEvaluate,
	BloomLevelCreate,
}

// DifficultyLevels defines standard difficulty levels
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// DifficultyLevels provides ordered list of difficulty levels
var DifficultyLevels = []string{
	DifficultyEasy,
	DifficultyMedium,
	DifficultyHard,
}

// BloomLevelWeights defines importance weights for each Bloom level
var BloomLevelWeights = map[string]float64{
	BloomLevelRemember:   1.0,
	BloomLevelUnderstand: 1.2,
	BloomLevelApply:      1.5,
	BloomLevelAnalyze:    1.8,
	BloomLevelEvaluate:   2.0,
	BloomLevelCreate:     2.5,
}

// IsValidBloomLevel checks if a given string is a valid Bloom taxonomy level
func IsValidBloomLevel(level string) bool {
	for _, validLevel := range BloomTaxonomyLevels {
		if level == validLevel {
			return true
		}
	}
	return false
}

// IsValidDifficultyLevel checks if a given string is a valid difficulty level
func IsValidDifficultyLevel(difficulty string) bool {
	for _, validDifficulty := range DifficultyLevels {
		if difficulty == validDifficulty {
			return true
		}
	}
	return false
}