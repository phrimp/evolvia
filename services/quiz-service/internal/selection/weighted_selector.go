package selection

import (
	"math"
	"math/rand"
	"quiz-service/internal/models"
	"slices"
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
			remaining = slices.Delete(remaining, idx, idx+1)
			continue
		}

		// Select based on weight
		r := s.rand.Float64() * totalWeight
		cumulative := 0.0

		for idx, wq := range remaining {
			cumulative += wq.Weight
			if r <= cumulative {
				selected = append(selected, wq)
				remaining = slices.Delete(remaining, idx, idx+1)
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
			remaining = slices.Delete(remaining, idx, idx+1)
			continue
		}

		r := s.rand.Float64() * totalWeight
		cumulative := 0.0

		for idx, wq := range remaining {
			cumulative += wq.Weight
			if r <= cumulative {
				selected = append(selected, wq)
				remaining = slices.Delete(remaining, idx, idx+1)
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
	return slices.Contains(excludeList, id)
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

func (s *WeightedSelector) SelectQuestionsWithEnhancedWeights(
	questions []models.Question,
	criteria *EnhancedSelectionCriteria,
	tagWeights TagWeightConfig,
) (*SelectionResult, error) {
	// Calculate enhanced weights for all questions
	weightedQuestions := s.calculateEnhancedWeights(questions, criteria, tagWeights)

	// Filter by minimum requirements if specified
	if criteria.MinPrimaryMatch > 0 || criteria.MinSecondaryMatch > 0 {
		weightedQuestions = s.filterByEnhancedRequirements(weightedQuestions, criteria)
	}

	if len(weightedQuestions) == 0 {
		return &SelectionResult{
			Questions:       []models.Question{},
			TotalCandidates: 0,
		}, nil
	}

	// Group by Bloom's level for balanced selection
	bloomGroups := s.groupEnhancedByBloomLevel(weightedQuestions)

	// Select questions maintaining Bloom's distribution
	selected := s.selectWithEnhancedDistribution(bloomGroups, criteria.BloomDistribution, criteria.Count)

	// Calculate average match score
	avgMatch := s.calculateEnhancedAverageMatch(selected)

	// Extract questions for result
	selectedQuestions := make([]models.Question, len(selected))
	for i, ewq := range selected {
		selectedQuestions[i] = ewq.Question
	}

	// Build detailed result
	result := &SelectionResult{
		Questions:       selectedQuestions,
		TotalCandidates: len(weightedQuestions),
		Weights:         s.convertToStandardWeighted(selected),
		AverageMatch:    avgMatch,
	}

	// Add bloom and tag coverage
	result.BloomCoverage = s.calculateBloomCoverage(selectedQuestions)
	result.TagCoverage = s.calculateTagCoverage(selectedQuestions)

	return result, nil
}

// calculateEnhancedWeights calculates weights using categorized tags
func (s *WeightedSelector) calculateEnhancedWeights(
	questions []models.Question,
	criteria *EnhancedSelectionCriteria,
	tagWeights TagWeightConfig,
) []EnhancedWeightedQuestion {
	weighted := make([]EnhancedWeightedQuestion, 0)
	skillInfo := criteria.SkillInfo

	for _, question := range questions {
		// Skip if excluded or wrong difficulty
		if s.isExcluded(question.ID, criteria.ExcludeIDs) {
			continue
		}
		if criteria.Difficulty != "" && question.DifficultyLevel != criteria.Difficulty {
			continue
		}

		// Calculate matches for each tag category
		primaryMatches, matchedPrimary := s.countTagMatches(question.TopicTags, skillInfo.PrimaryTags)
		secondaryMatches, matchedSecondary := s.countTagMatches(question.TopicTags, skillInfo.SecondaryTags)
		relatedMatches, matchedRelated := s.countTagMatches(question.TopicTags, skillInfo.RelatedTags)

		// Calculate weighted tag score
		// This is the KEY FORMULA that uses the weight ratios
		tagScore := s.calculateWeightedTagScore(
			primaryMatches,
			secondaryMatches,
			relatedMatches,
			tagWeights,
			criteria.WeightExponent,
		)

		// Apply skill match bonus
		skillBonus := 0.0
		if criteria.PreferExactSkill && question.SkillID == skillInfo.ID {
			skillBonus = tagWeights.ExactMatchBonus
		}

		// Apply Bloom's level weight
		bloomWeight := s.getBloomWeight(question.BloomLevel, criteria.BloomDistribution)

		// Calculate final weight (multiplicative combination)
		totalWeight := tagScore * bloomWeight * (1.0 + skillBonus)

		// Ensure minimum weight for questions with no matches
		if totalWeight < 0.1 {
			totalWeight = 0.1
		}

		ewq := EnhancedWeightedQuestion{
			Question:         question,
			TotalWeight:      totalWeight,
			PrimaryMatches:   primaryMatches,
			SecondaryMatches: secondaryMatches,
			RelatedMatches:   relatedMatches,
			MatchedPrimary:   matchedPrimary,
			MatchedSecondary: matchedSecondary,
			MatchedRelated:   matchedRelated,
			TagWeight:        tagScore,
			BloomWeight:      bloomWeight,
			SkillMatchBonus:  skillBonus,
			WeightComponents: map[string]float64{
				"tag_score":   tagScore,
				"bloom":       bloomWeight,
				"skill_bonus": skillBonus,
				"total":       totalWeight,
			},
		}

		weighted = append(weighted, ewq)
	}

	return weighted
}

// calculateWeightedTagScore is the CORE FORMULA for tag weight calculation
func (s *WeightedSelector) calculateWeightedTagScore(
	primaryMatches, secondaryMatches, relatedMatches int,
	weights TagWeightConfig,
	exponent float64,
) float64 {
	// Calculate weighted sum using the configured ratios
	// This ensures primary tags have higher impact than secondary, and secondary than related
	weightedSum := float64(primaryMatches)*weights.PrimaryWeight +
		float64(secondaryMatches)*weights.SecondaryWeight +
		float64(relatedMatches)*weights.RelatedWeight

	if weightedSum == 0 {
		return 0.1 // Minimal weight for no matches
	}

	// Apply exponent to emphasize questions with more weighted matches
	// This makes questions with multiple primary matches much more likely to be selected
	return math.Pow(weightedSum, exponent)
}

// filterByEnhancedRequirements filters by minimum tag match requirements
func (s *WeightedSelector) filterByEnhancedRequirements(
	questions []EnhancedWeightedQuestion,
	criteria *EnhancedSelectionCriteria,
) []EnhancedWeightedQuestion {
	filtered := make([]EnhancedWeightedQuestion, 0)

	for _, q := range questions {
		if q.PrimaryMatches >= criteria.MinPrimaryMatch &&
			q.SecondaryMatches >= criteria.MinSecondaryMatch {
			filtered = append(filtered, q)
		}
	}

	// If too strict, relax requirements
	if len(filtered) < criteria.Count {
		// Try with just primary requirement
		if criteria.MinPrimaryMatch > 0 {
			filtered = make([]EnhancedWeightedQuestion, 0)
			for _, q := range questions {
				if q.PrimaryMatches >= criteria.MinPrimaryMatch {
					filtered = append(filtered, q)
				}
			}
		}

		// If still not enough, return all
		if len(filtered) < criteria.Count {
			return questions
		}
	}

	return filtered
}

// groupEnhancedByBloomLevel groups enhanced weighted questions by Bloom level
func (s *WeightedSelector) groupEnhancedByBloomLevel(
	questions []EnhancedWeightedQuestion,
) map[string][]EnhancedWeightedQuestion {
	groups := make(map[string][]EnhancedWeightedQuestion)

	for _, q := range questions {
		level := strings.ToLower(q.Question.BloomLevel)
		groups[level] = append(groups[level], q)
	}

	return groups
}

// selectWithEnhancedDistribution selects maintaining Bloom distribution
func (s *WeightedSelector) selectWithEnhancedDistribution(
	groups map[string][]EnhancedWeightedQuestion,
	distribution map[string]float64,
	totalCount int,
) []EnhancedWeightedQuestion {
	selected := make([]EnhancedWeightedQuestion, 0, totalCount)

	// Calculate how many questions per Bloom level
	levelCounts := s.calculateLevelCounts(distribution, totalCount)

	// Select from each level
	for level, count := range levelCounts {
		if levelQuestions, exists := groups[level]; exists && len(levelQuestions) > 0 {
			// Sort by weight within level
			sort.Slice(levelQuestions, func(i, j int) bool {
				return levelQuestions[i].TotalWeight > levelQuestions[j].TotalWeight
			})

			// Select using weighted random
			toSelect := min(count, len(levelQuestions))
			if toSelect > 0 {
				levelSelected := s.weightedRandomSelectEnhanced(levelQuestions, toSelect)
				selected = append(selected, levelSelected...)
			}
		}
	}

	// Fill remaining slots if needed
	if len(selected) < totalCount {
		remaining := s.getAllRemainingEnhanced(groups, selected)
		if len(remaining) > 0 {
			additional := s.weightedRandomSelectEnhanced(remaining, totalCount-len(selected))
			selected = append(selected, additional...)
		}
	}

	return selected
}

// weightedRandomSelectEnhanced performs weighted random selection on enhanced questions
func (s *WeightedSelector) weightedRandomSelectEnhanced(
	questions []EnhancedWeightedQuestion,
	count int,
) []EnhancedWeightedQuestion {
	if len(questions) <= count {
		return questions
	}

	selected := make([]EnhancedWeightedQuestion, 0, count)
	remaining := make([]EnhancedWeightedQuestion, len(questions))
	copy(remaining, questions)

	for i := 0; i < count && len(remaining) > 0; i++ {
		// Calculate total weight
		totalWeight := 0.0
		for _, q := range remaining {
			totalWeight += q.TotalWeight
		}

		if totalWeight == 0 {
			// Random selection if all weights are 0
			idx := s.rand.Intn(len(remaining))
			selected = append(selected, remaining[idx])
			remaining = append(remaining[:idx], remaining[idx+1:]...)
			continue
		}

		// Weighted random selection
		r := s.rand.Float64() * totalWeight
		cumulative := 0.0

		for idx, q := range remaining {
			cumulative += q.TotalWeight
			if r <= cumulative {
				selected = append(selected, q)
				remaining = append(remaining[:idx], remaining[idx+1:]...)
				break
			}
		}
	}

	return selected
}

// Helper methods for enhanced selection

func (s *WeightedSelector) getAllRemainingEnhanced(
	groups map[string][]EnhancedWeightedQuestion,
	selected []EnhancedWeightedQuestion,
) []EnhancedWeightedQuestion {
	selectedIDs := make(map[string]bool)
	for _, q := range selected {
		selectedIDs[q.Question.ID] = true
	}

	var remaining []EnhancedWeightedQuestion
	for _, levelQuestions := range groups {
		for _, q := range levelQuestions {
			if !selectedIDs[q.Question.ID] {
				remaining = append(remaining, q)
			}
		}
	}

	return remaining
}

func (s *WeightedSelector) calculateEnhancedAverageMatch(
	questions []EnhancedWeightedQuestion,
) float64 {
	if len(questions) == 0 {
		return 0
	}

	// Calculate weighted average based on tag importance
	totalWeightedMatches := 0.0
	for _, q := range questions {
		// Use the same weight ratios for averaging
		weightedMatches := float64(q.PrimaryMatches)*3.0 +
			float64(q.SecondaryMatches)*1.5 +
			float64(q.RelatedMatches)*0.5
		totalWeightedMatches += weightedMatches
	}

	return totalWeightedMatches / float64(len(questions))
}

func (s *WeightedSelector) convertToStandardWeighted(
	enhanced []EnhancedWeightedQuestion,
) []WeightedQuestion {
	standard := make([]WeightedQuestion, len(enhanced))

	for i, ewq := range enhanced {
		// Merge all matched tags
		allMatched := make([]string, 0)
		allMatched = append(allMatched, ewq.MatchedPrimary...)
		allMatched = append(allMatched, ewq.MatchedSecondary...)
		allMatched = append(allMatched, ewq.MatchedRelated...)

		standard[i] = WeightedQuestion{
			Question:    ewq.Question,
			Weight:      ewq.TotalWeight,
			TagMatches:  ewq.PrimaryMatches + ewq.SecondaryMatches + ewq.RelatedMatches,
			MatchedTags: allMatched,
			BloomLevel:  ewq.Question.BloomLevel,
			TagWeight:   ewq.TagWeight,
			BloomWeight: ewq.BloomWeight,
		}
	}

	return standard
}

func (s *WeightedSelector) calculateBloomCoverage(questions []models.Question) map[string]int {
	coverage := make(map[string]int)
	for _, q := range questions {
		coverage[q.BloomLevel]++
	}
	return coverage
}

func (s *WeightedSelector) calculateTagCoverage(questions []models.Question) map[string]int {
	coverage := make(map[string]int)
	for _, q := range questions {
		for _, tag := range q.TopicTags {
			coverage[tag]++
		}
	}
	return coverage
}
