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

// SelectQuestionsWithBloom selects questions based on weighted tag matching and Bloom's level
func (s *WeightedSelector) SelectQuestionsWithBloom(
	questions []models.Question,
	criteria *SelectionCriteria,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// Filter and weight questions
	weightedQuestions := s.calculateWeightsWithBloom(questions, criteria, bloomDistribution)

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

	// Group by Bloom's level for balanced selection
	bloomGroups := s.groupByBloomLevel(weightedQuestions)

	// Select questions maintaining Bloom's distribution
	selected := s.selectWithBloomDistribution(bloomGroups, bloomDistribution, criteria.Count)

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

// calculateWeightsWithBloom calculates weight for each question based on tag matches and Bloom's level
func (s *WeightedSelector) calculateWeightsWithBloom(
	questions []models.Question,
	criteria *SelectionCriteria,
	bloomDistribution map[string]float64,
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

		// Calculate base weight from tag matches
		tagWeight := s.calculateWeight(matches, criteria.WeightExponent)

		// Apply Bloom's level weight modifier
		bloomWeight := s.getBloomWeight(question.BloomLevel, bloomDistribution)

		// Combine weights (multiplicative approach)
		finalWeight := tagWeight * bloomWeight

		weighted = append(weighted, WeightedQuestion{
			Question:    question,
			Weight:      finalWeight,
			TagMatches:  matches,
			MatchedTags: matchedTags,
			BloomLevel:  question.BloomLevel,
			TagWeight:   tagWeight,
			BloomWeight: bloomWeight,
		})
	}

	return weighted
}

// getBloomWeight returns weight modifier based on Bloom's level preference
func (s *WeightedSelector) getBloomWeight(bloomLevel string, distribution map[string]float64) float64 {
	if weight, exists := distribution[strings.ToLower(bloomLevel)]; exists {
		// Scale the weight (0.1 to 2.0 range)
		// Higher distribution percentage = higher weight
		return 0.1 + (weight * 1.9)
	}
	// Default weight for unspecified levels
	return 0.1
}

// groupByBloomLevel groups weighted questions by their Bloom's taxonomy level
func (s *WeightedSelector) groupByBloomLevel(questions []WeightedQuestion) map[string][]WeightedQuestion {
	groups := make(map[string][]WeightedQuestion)

	for _, q := range questions {
		level := strings.ToLower(q.BloomLevel)
		groups[level] = append(groups[level], q)
	}

	return groups
}

// selectWithBloomDistribution selects questions maintaining desired Bloom's distribution
func (s *WeightedSelector) selectWithBloomDistribution(
	groups map[string][]WeightedQuestion,
	distribution map[string]float64,
	totalCount int,
) []WeightedQuestion {
	selected := make([]WeightedQuestion, 0, totalCount)

	// Calculate how many questions to select from each Bloom level
	levelCounts := s.calculateLevelCounts(distribution, totalCount)

	// Select questions from each level
	for level, count := range levelCounts {
		if levelQuestions, exists := groups[level]; exists && len(levelQuestions) > 0 {
			// Sort by weight within the level
			sort.Slice(levelQuestions, func(i, j int) bool {
				return levelQuestions[i].Weight > levelQuestions[j].Weight
			})

			// Select top weighted questions from this level
			toSelect := min(count, len(levelQuestions))
			if toSelect > 0 {
				levelSelected := s.weightedRandomSelectFromGroup(levelQuestions, toSelect)
				selected = append(selected, levelSelected...)
			}
		}
	}

	// If we haven't selected enough, fill from any available questions
	if len(selected) < totalCount {
		remaining := totalCount - len(selected)
		allRemaining := s.getAllRemainingQuestions(groups, selected)
		if len(allRemaining) > 0 {
			additional := s.weightedRandomSelect(allRemaining, remaining)
			selected = append(selected, additional...)
		}
	}

	return selected
}

// calculateLevelCounts determines how many questions to select from each Bloom level
func (s *WeightedSelector) calculateLevelCounts(distribution map[string]float64, total int) map[string]int {
	counts := make(map[string]int)
	allocated := 0

	// First pass: allocate based on percentages
	for level, percentage := range distribution {
		count := int(math.Round(percentage * float64(total)))
		counts[level] = count
		allocated += count
	}

	// Adjust if we've over or under allocated
	if allocated != total {
		// Find the level with highest percentage to adjust
		maxLevel := ""
		maxPercentage := 0.0
		for level, percentage := range distribution {
			if percentage > maxPercentage {
				maxPercentage = percentage
				maxLevel = level
			}
		}
		if maxLevel != "" {
			counts[maxLevel] += (total - allocated)
		}
	}

	return counts
}

// weightedRandomSelectFromGroup performs weighted selection within a group
func (s *WeightedSelector) weightedRandomSelectFromGroup(
	questions []WeightedQuestion,
	count int,
) []WeightedQuestion {
	if len(questions) <= count {
		return questions
	}

	selected := make([]WeightedQuestion, 0, count)
	remaining := make([]WeightedQuestion, len(questions))
	copy(remaining, questions)

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
				remaining = append(remaining[:idx], remaining[idx+1:]...)
				break
			}
		}
	}

	return selected
}

// getAllRemainingQuestions gets all questions not yet selected
func (s *WeightedSelector) getAllRemainingQuestions(
	groups map[string][]WeightedQuestion,
	selected []WeightedQuestion,
) []WeightedQuestion {
	selectedIDs := make(map[string]bool)
	for _, q := range selected {
		selectedIDs[q.Question.ID] = true
	}

	var remaining []WeightedQuestion
	for _, levelQuestions := range groups {
		for _, q := range levelQuestions {
			if !selectedIDs[q.Question.ID] {
				remaining = append(remaining, q)
			}
		}
	}

	return remaining
}

// Legacy method for backward compatibility
func (s *WeightedSelector) SelectQuestions(
	questions []models.Question,
	criteria *SelectionCriteria,
) (*SelectionResult, error) {
	// Use uniform distribution for Bloom's levels when not specified
	uniformDist := map[string]float64{
		"remember":   0.17,
		"understand": 0.17,
		"apply":      0.17,
		"analyze":    0.17,
		"evaluate":   0.16,
		"create":     0.16,
	}
	return s.SelectQuestionsWithBloom(questions, criteria, uniformDist)
}

// Helper function for minimum value
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ... (preserve other existing methods like countTagMatches, calculateWeight, etc.)

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
	return math.Pow(float64(matches), exponent)
}

// weightedRandomSelect performs weighted random selection
func (s *WeightedSelector) weightedRandomSelect(
	weighted []WeightedQuestion,
	count int,
) []WeightedQuestion {
	if len(weighted) <= count {
		return weighted
	}

	selected := make([]WeightedQuestion, 0, count)
	remaining := make([]WeightedQuestion, len(weighted))
	copy(remaining, weighted)

	for i := 0; i < count && len(remaining) > 0; i++ {
		totalWeight := 0.0
		for _, wq := range remaining {
			totalWeight += wq.Weight
		}

		if totalWeight == 0 {
			idx := s.rand.Intn(len(remaining))
			selected = append(selected, remaining[idx])
			remaining = append(remaining[:idx], remaining[idx+1:]...)
			continue
		}

		r := s.rand.Float64() * totalWeight
		cumulative := 0.0

		for idx, wq := range remaining {
			cumulative += wq.Weight
			if r <= cumulative {
				selected = append(selected, wq)
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

	sort.Slice(weighted, func(i, j int) bool {
		return weighted[i].TagMatches > weighted[j].TagMatches
	})

	if limit > len(weighted) {
		limit = len(weighted)
	}

	return weighted[:limit]
}
