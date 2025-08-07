package selection

import (
	"math"
	"math/rand"
	"quiz-service/internal/models"
	"sort"
	"strings"
	"time"
)

// WeightedSelector handles weighted random selection of questions
type WeightedSelector struct {
	rand *rand.Rand
}

// NewWeightedSelector creates a new weighted selector
func NewWeightedSelector() *WeightedSelector {
	return &WeightedSelector{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectQuestions selects questions based on weighted tag matching
func (s *WeightedSelector) SelectQuestions(
	questions []models.Question,
	criteria *SelectionCriteria,
) (*SelectionResult, error) {
	// Filter and weight questions
	weightedQuestions := s.calculateWeights(questions, criteria)

	// Filter by minimum tag match if specified
	if criteria.MinTagMatch > 0 {
		weightedQuestions = s.filterByMinMatch(weightedQuestions, criteria.MinTagMatch)
	}

	if len(weightedQuestions) == 0 {
		return &SelectionResult{
			Questions:       []models.Question{},
			TotalCandidates: 0,
		}, nil
	}

	// Select questions using weighted random selection
	selected := s.weightedRandomSelect(weightedQuestions, criteria.Count)

	// Calculate average match score
	avgMatch := s.calculateAverageMatch(selected)

	// Extract just the questions for the result
	selectedQuestions := make([]models.Question, len(selected))
	for i, wq := range selected {
		selectedQuestions[i] = wq.Question
	}

	return &SelectionResult{
		Questions:       selectedQuestions,
		TotalCandidates: len(weightedQuestions),
		Weights:         selected,
		AverageMatch:    avgMatch,
	}, nil
}

// calculateWeights calculates weight for each question based on tag matches
func (s *WeightedSelector) calculateWeights(
	questions []models.Question,
	criteria *SelectionCriteria,
) []WeightedQuestion {
	weighted := make([]WeightedQuestion, 0)

	for _, question := range questions {
		// Skip if in exclude list
		if s.isExcluded(question.ID, criteria.ExcludeIDs) {
			continue
		}

		// Skip if difficulty doesn't match
		if criteria.Difficulty != "" && question.DifficultyLevel != criteria.Difficulty {
			continue
		}

		// Calculate tag matches
		matches, matchedTags := s.countTagMatches(question.TopicTags, criteria.SkillTags)

		// Calculate weight based on matches
		weight := s.calculateWeight(matches, criteria.WeightExponent)

		weighted = append(weighted, WeightedQuestion{
			Question:    question,
			Weight:      weight,
			TagMatches:  matches,
			MatchedTags: matchedTags,
		})
	}

	return weighted
}

// countTagMatches counts how many tags match between question and skill
func (s *WeightedSelector) countTagMatches(questionTags, skillTags []string) (int, []string) {
	matches := 0
	matchedTags := []string{}

	// Create map for O(1) lookup
	skillTagMap := make(map[string]bool)
	for _, tag := range skillTags {
		skillTagMap[strings.ToLower(tag)] = true
	}

	// Count matches
	for _, qTag := range questionTags {
		if skillTagMap[strings.ToLower(qTag)] {
			matches++
			matchedTags = append(matchedTags, qTag)
		}
	}

	return matches, matchedTags
}

// calculateWeight calculates selection weight based on match count
func (s *WeightedSelector) calculateWeight(matches int, exponent float64) float64 {
	if matches == 0 {
		// Give minimal weight to questions with no matches
		return 0.1
	}
	// Use exponential weighting to prefer questions with more matches
	// e.g., with exponent 2: 1 match = 1, 2 matches = 4, 3 matches = 9
	return math.Pow(float64(matches), exponent)
}

// weightedRandomSelect performs weighted random selection
func (s *WeightedSelector) weightedRandomSelect(
	weighted []WeightedQuestion,
	count int,
) []WeightedQuestion {
	if len(weighted) <= count {
		// Return all if we have fewer than requested
		return weighted
	}

	selected := make([]WeightedQuestion, 0, count)
	remaining := make([]WeightedQuestion, len(weighted))
	copy(remaining, weighted)

	for i := 0; i < count && len(remaining) > 0; i++ {
		// Calculate total weight
		totalWeight := 0.0
		for _, wq := range remaining {
			totalWeight += wq.Weight
		}

		if totalWeight == 0 {
			// If all weights are 0, select randomly
			idx := s.rand.Intn(len(remaining))
			selected = append(selected, remaining[idx])
			remaining = append(remaining[:idx], remaining[idx+1:]...)
			continue
		}

		// Select based on weight
		r := s.rand.Float64() * totalWeight
		cumulative := 0.0

		for idx, wq := range remaining {
			cumulative += wq.Weight
			if r <= cumulative {
				selected = append(selected, wq)
				// Remove selected item
				remaining = append(remaining[:idx], remaining[idx+1:]...)
				break
			}
		}
	}

	return selected
}

// filterByMinMatch filters questions that have minimum tag matches
func (s *WeightedSelector) filterByMinMatch(
	weighted []WeightedQuestion,
	minMatch int,
) []WeightedQuestion {
	filtered := make([]WeightedQuestion, 0)
	for _, wq := range weighted {
		if wq.TagMatches >= minMatch {
			filtered = append(filtered, wq)
		}
	}
	return filtered
}

// isExcluded checks if a question ID is in the exclude list
func (s *WeightedSelector) isExcluded(id string, excludeList []string) bool {
	for _, excludeID := range excludeList {
		if id == excludeID {
			return true
		}
	}
	return false
}

// calculateAverageMatch calculates average tag match score
func (s *WeightedSelector) calculateAverageMatch(weighted []WeightedQuestion) float64 {
	if len(weighted) == 0 {
		return 0
	}

	total := 0
	for _, wq := range weighted {
		total += wq.TagMatches
	}

	return float64(total) / float64(len(weighted))
}

// GetTopMatchingQuestions returns questions sorted by match score (for debugging/analysis)
func (s *WeightedSelector) GetTopMatchingQuestions(
	questions []models.Question,
	skillTags []string,
	limit int,
) []WeightedQuestion {
	// Calculate weights for all questions
	weighted := make([]WeightedQuestion, 0)
	for _, q := range questions {
		matches, matchedTags := s.countTagMatches(q.TopicTags, skillTags)
		weighted = append(weighted, WeightedQuestion{
			Question:    q,
			Weight:      float64(matches),
			TagMatches:  matches,
			MatchedTags: matchedTags,
		})
	}

	// Sort by tag matches (descending)
	sort.Slice(weighted, func(i, j int) bool {
		return weighted[i].TagMatches > weighted[j].TagMatches
	})

	// Return top N
	if limit > len(weighted) {
		limit = len(weighted)
	}

	return weighted[:limit]
}
