package event

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/services"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
)

// NewSkillDiscovery handles automatic detection of new skills from content
type NewSkillDiscovery struct {
	skillService        *services.SkillService
	confidenceThreshold float64
	minFrequency        int
	techKeywords        []string
	skillPatterns       []*regexp.Regexp
}

// SkillCandidate represents a potential new skill discovered in text
type SkillCandidate struct {
	Term            string            `json:"term"`
	Frequency       int               `json:"frequency"`
	Contexts        []string          `json:"contexts"`
	ConfidenceScore float64           `json:"confidence_score"`
	Category        string            `json:"category"`
	Sources         map[string]int    `json:"sources"` // source -> frequency
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	SuggestedLevel  models.SkillLevel `json:"suggested_level"`
}

func NewSkillDiscoveryService() *NewSkillDiscovery {
	return &NewSkillDiscovery{
		confidenceThreshold: 0.7,
		minFrequency:        3,
		techKeywords: []string{
			"programming", "framework", "library", "tool", "platform",
			"language", "database", "api", "sdk", "ide", "version control",
			"testing", "deployment", "cloud", "devops", "agile", "methodology",
		},
		skillPatterns: []*regexp.Regexp{
			// Programming languages
			regexp.MustCompile(`(?i)\b([A-Z][a-z]*(?:\+\+|#|\.js|\.py)?)\s+(?:programming|language|development)\b`),
			// Frameworks and libraries
			regexp.MustCompile(`(?i)\b(React|Angular|Vue|Django|Flask|Spring|Laravel|Express)(?:\s+(?:framework|library))?\b`),
			// Tools and platforms
			regexp.MustCompile(`(?i)\b(Docker|Kubernetes|Jenkins|Git|AWS|Azure|GCP)\s+(?:experience|skills|knowledge)\b`),
			// Methodologies
			regexp.MustCompile(`(?i)\b(Agile|Scrum|Kanban|DevOps|CI/CD)\s+(?:methodology|practices|experience)\b`),
			// Certifications and technologies
			regexp.MustCompile(`(?i)\b([A-Z]{2,}(?:\s+[A-Z]{2,})*)\s+(?:certified|certification|expertise)\b`),
		},
	}
}

// DiscoverNewSkills analyzes text content to identify potential new skills
func (nsd *NewSkillDiscovery) DiscoverNewSkills(ctx context.Context, content string, source string) ([]*SkillCandidate, error) {
	// Step 1: Extract potential skill terms using multiple techniques
	candidates := make(map[string]*SkillCandidate)

	// Technique 1: Pattern-based extraction
	patternCandidates := nsd.extractByPatterns(content)
	nsd.mergeCandidates(candidates, patternCandidates, source)

	// Technique 2: N-gram analysis for technical terms
	ngramCandidates := nsd.extractByNGrams(content, source)
	nsd.mergeCandidates(candidates, ngramCandidates, source)

	// Technique 3: Context-based extraction (terms near skill indicators)
	contextCandidates := nsd.extractByContext(content, source)
	nsd.mergeCandidates(candidates, contextCandidates, source)

	// Step 2: Filter and score candidates
	filteredCandidates := nsd.filterAndScore(candidates, content)

	// Step 3: Check against existing skills to avoid duplicates
	newCandidates := []*SkillCandidate{}
	for _, candidate := range filteredCandidates {
		if !nsd.isExistingSkill(ctx, candidate.Term) {
			newCandidates = append(newCandidates, candidate)
		}
	}

	// Step 4: Sort by confidence score
	sort.Slice(newCandidates, func(i, j int) bool {
		return newCandidates[i].ConfidenceScore > newCandidates[j].ConfidenceScore
	})

	return newCandidates, nil
}

// extractByPatterns uses regex patterns to find skill-like terms
func (nsd *NewSkillDiscovery) extractByPatterns(content string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	for _, pattern := range nsd.skillPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				term := strings.TrimSpace(match[1])
				if len(term) > 2 && len(term) < 50 { // Reasonable skill name length
					if existing, exists := candidates[term]; exists {
						existing.Frequency++
					} else {
						candidates[term] = &SkillCandidate{
							Term:      term,
							Frequency: 1,
							Contexts:  []string{match[0]},
							FirstSeen: time.Now(),
							LastSeen:  time.Now(),
							Sources:   make(map[string]int),
						}
					}
				}
			}
		}
	}

	return candidates
}

// extractByNGrams analyzes n-grams to find technical terms
func (nsd *NewSkillDiscovery) extractByNGrams(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	// Clean and tokenize
	words := nsd.tokenize(content)

	// Extract 1-grams, 2-grams, and 3-grams
	for n := 1; n <= 3; n++ {
		ngrams := nsd.generateNGrams(words, n)
		for _, ngram := range ngrams {
			if nsd.isLikelySkill(ngram) {
				term := strings.Join(ngram, " ")
				if existing, exists := candidates[term]; exists {
					existing.Frequency++
				} else {
					candidates[term] = &SkillCandidate{
						Term:      term,
						Frequency: 1,
						FirstSeen: time.Now(),
						LastSeen:  time.Now(),
						Sources:   map[string]int{source: 1},
					}
				}
			}
		}
	}

	return candidates
}

// extractByContext looks for terms appearing near skill indicator words
func (nsd *NewSkillDiscovery) extractByContext(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	skillIndicators := []string{
		"experience with", "skilled in", "proficient in", "expertise in",
		"knowledge of", "familiar with", "worked with", "using", "developed with",
		"certified in", "specializing in", "background in",
	}

	sentences := strings.Split(content, ".")
	for _, sentence := range sentences {
		sentence = strings.ToLower(strings.TrimSpace(sentence))

		for _, indicator := range skillIndicators {
			if strings.Contains(sentence, indicator) {
				// Extract terms after the indicator
				parts := strings.Split(sentence, indicator)
				if len(parts) > 1 {
					afterIndicator := strings.TrimSpace(parts[1])
					terms := nsd.extractTermsFromPhrase(afterIndicator)

					for _, term := range terms {
						if nsd.isValidSkillTerm(term) {
							if existing, exists := candidates[term]; exists {
								existing.Frequency++
								existing.Contexts = append(existing.Contexts, sentence)
							} else {
								candidates[term] = &SkillCandidate{
									Term:      term,
									Frequency: 1,
									Contexts:  []string{sentence},
									FirstSeen: time.Now(),
									LastSeen:  time.Now(),
									Sources:   map[string]int{source: 1},
								}
							}
						}
					}
				}
			}
		}
	}

	return candidates
}

// mergeCandidates combines candidate maps
func (nsd *NewSkillDiscovery) mergeCandidates(target map[string]*SkillCandidate, source map[string]*SkillCandidate, sourceName string) {
	for term, candidate := range source {
		if existing, exists := target[term]; exists {
			existing.Frequency += candidate.Frequency
			existing.Contexts = append(existing.Contexts, candidate.Contexts...)
			if sourceName != "" {
				existing.Sources[sourceName]++
			}
			existing.LastSeen = time.Now()
		} else {
			if sourceName != "" {
				candidate.Sources[sourceName] = candidate.Frequency
			}
			target[term] = candidate
		}
	}
}

// filterAndScore applies filtering and confidence scoring
func (nsd *NewSkillDiscovery) filterAndScore(candidates map[string]*SkillCandidate, content string) []*SkillCandidate {
	var filtered []*SkillCandidate

	for _, candidate := range candidates {
		// Apply minimum frequency filter
		if candidate.Frequency < nsd.minFrequency {
			continue
		}

		// Calculate confidence score
		confidence := nsd.calculateConfidence(candidate, content)
		candidate.ConfidenceScore = confidence

		// Apply confidence threshold
		if confidence >= nsd.confidenceThreshold {
			// Categorize and suggest level
			candidate.Category = nsd.categorizeSkill(candidate.Term)
			candidate.SuggestedLevel = nsd.suggestSkillLevel(candidate, content)

			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

// calculateConfidence assigns a confidence score to a skill candidate
func (nsd *NewSkillDiscovery) calculateConfidence(candidate *SkillCandidate, content string) float64 {
	score := 0.0

	// Base score from frequency (normalized)
	score += float64(candidate.Frequency) * 0.1

	// Bonus for appearing in multiple sources
	if len(candidate.Sources) > 1 {
		score += 0.2
	}

	// Bonus for technical context
	for _, context := range candidate.Contexts {
		for _, keyword := range nsd.techKeywords {
			if strings.Contains(strings.ToLower(context), keyword) {
				score += 0.1
				break
			}
		}
	}

	// Bonus for proper capitalization (likely proper nouns/brand names)
	if nsd.isProperlyCapitalized(candidate.Term) {
		score += 0.15
	}

	// Bonus for version numbers or technical suffixes
	if nsd.hasTechnicalSuffix(candidate.Term) {
		score += 0.1
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// isExistingSkill checks if a term already exists in the skill database
func (nsd *NewSkillDiscovery) isExistingSkill(ctx context.Context, term string) bool {
	// Check exact name match
	skill, _ := nsd.skillService.GetSkillByName(ctx, term)
	if skill != nil {
		return true
	}

	// Check common names and variations
	skills, _ := nsd.skillService.SearchSkills(ctx, term, 10)
	for _, skill := range skills {
		if strings.EqualFold(skill.Name, term) {
			return true
		}
		for _, commonName := range skill.CommonNames {
			if strings.EqualFold(commonName, term) {
				return true
			}
		}
	}

	return false
}

// Helper functions for text processing and analysis
func (nsd *NewSkillDiscovery) tokenize(text string) []string {
	// Simple tokenization - can be enhanced with NLP libraries
	words := strings.Fields(strings.ToLower(text))
	var cleaned []string
	for _, word := range words {
		// Remove punctuation and keep only alphanumeric
		word = regexp.MustCompile(`[^a-zA-Z0-9\+\#\.]`).ReplaceAllString(word, "")
		if len(word) > 1 {
			cleaned = append(cleaned, word)
		}
	}
	return cleaned
}

func (nsd *NewSkillDiscovery) generateNGrams(words []string, n int) [][]string {
	var ngrams [][]string
	for i := 0; i <= len(words)-n; i++ {
		ngrams = append(ngrams, words[i:i+n])
	}
	return ngrams
}

func (nsd *NewSkillDiscovery) isLikelySkill(ngram []string) bool {
	term := strings.Join(ngram, " ")

	// Filter out common words
	stopWords := []string{"the", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by"}
	for _, word := range ngram {
		for _, stop := range stopWords {
			if word == stop {
				return false
			}
		}
	}

	// Must contain at least one capitalized word or technical term
	hasCapitalized := false
	for _, word := range ngram {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			hasCapitalized = true
			break
		}
	}

	return hasCapitalized && len(term) >= 3 && len(term) <= 30
}

func (nsd *NewSkillDiscovery) isValidSkillTerm(term string) bool {
	term = strings.TrimSpace(term)
	return len(term) >= 2 && len(term) <= 50 && !nsd.isCommonWord(term)
}

func (nsd *NewSkillDiscovery) isCommonWord(term string) bool {
	commonWords := []string{
		"experience", "knowledge", "skills", "ability", "strong", "good", "excellent",
		"years", "months", "project", "projects", "work", "working", "development",
	}

	for _, common := range commonWords {
		if strings.EqualFold(term, common) {
			return true
		}
	}
	return false
}

func (nsd *NewSkillDiscovery) extractTermsFromPhrase(phrase string) []string {
	// Split on common delimiters
	delimiters := regexp.MustCompile(`[,;\n\r]+`)
	parts := delimiters.Split(phrase, -1)

	var terms []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) > 0 {
			// Further split on "and", "or"
			words := regexp.MustCompile(`\s+(?:and|or)\s+`).Split(part, -1)
			for _, word := range words {
				word = strings.TrimSpace(word)
				if nsd.isValidSkillTerm(word) {
					terms = append(terms, word)
				}
			}
		}
	}

	return terms
}

func (nsd *NewSkillDiscovery) isProperlyCapitalized(term string) bool {
	words := strings.Fields(term)
	for _, word := range words {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			return true
		}
	}
	return false
}

func (nsd *NewSkillDiscovery) hasTechnicalSuffix(term string) bool {
	technicalSuffixes := []string{".js", ".py", ".java", "++", "#", ".net", ".0", "2.0", "3.0"}
	for _, suffix := range technicalSuffixes {
		if strings.HasSuffix(strings.ToLower(term), suffix) {
			return true
		}
	}
	return false
}

func (nsd *NewSkillDiscovery) categorizeSkill(term string) string {
	categories := map[string][]string{
		"Programming Languages": {"python", "java", "javascript", "c++", "go", "rust", "swift"},
		"Frameworks":            {"react", "angular", "vue", "django", "spring", "express"},
		"Cloud Platforms":       {"aws", "azure", "gcp", "kubernetes", "docker"},
		"Databases":             {"mysql", "postgresql", "mongodb", "redis", "elasticsearch"},
		"Tools":                 {"git", "jenkins", "jira", "confluence", "slack"},
	}

	termLower := strings.ToLower(term)
	for category, keywords := range categories {
		for _, keyword := range keywords {
			if strings.Contains(termLower, keyword) {
				return category
			}
		}
	}

	return "General"
}

func (nsd *NewSkillDiscovery) suggestSkillLevel(candidate *SkillCandidate, content string) models.SkillLevel {
	// Analyze context to suggest appropriate skill level
	contentLower := strings.ToLower(content)

	expertIndicators := []string{"expert", "senior", "lead", "architect", "advanced"}
	intermediateIndicators := []string{"intermediate", "experienced", "proficient"}
	beginnerIndicators := []string{"beginner", "basic", "fundamental", "learning"}

	for _, indicator := range expertIndicators {
		if strings.Contains(contentLower, indicator) {
			return models.SkillLevelExpert
		}
	}

	for _, indicator := range intermediateIndicators {
		if strings.Contains(contentLower, indicator) {
			return models.SkillLevelIntermediate
		}
	}

	for _, indicator := range beginnerIndicators {
		if strings.Contains(contentLower, indicator) {
			return models.SkillLevelBeginner
		}
	}

	// Default to beginner for safety
	return models.SkillLevelBeginner
}

// AutoAddNewSkills automatically creates skill entries for high-confidence candidates
func (nsd *NewSkillDiscovery) AutoAddNewSkills(ctx context.Context, candidates []*SkillCandidate, autoAddThreshold float64) error {
	for _, candidate := range candidates {
		if candidate.ConfidenceScore >= autoAddThreshold {
			skill := &models.Skill{
				Name:        candidate.Term,
				Description: fmt.Sprintf("Automatically discovered skill: %s", candidate.Term),
				CommonNames: []string{candidate.Term},
				IdentificationRules: models.SkillIdentificationRules{
					PrimaryPatterns: []models.KeywordPattern{
						{
							Text:   candidate.Term,
							Weight: 0.8,
							Type:   "exact",
						},
					},
					MinPrimaryMatches: 1,
					MinTotalScore:     0.3,
				},
				Tags: []string{"auto-discovered", candidate.Category},
				Metadata: models.SkillMetadata{
					Difficulty:   5,  // Default medium difficulty
					TimeToLearn:  40, // Default 40 hours
					Trending:     true,
					MarketDemand: candidate.ConfidenceScore,
				},
			}

			_, err := nsd.skillService.CreateSkill(ctx, skill)
			if err != nil {
				log.Printf("Failed to auto-add skill '%s': %v", candidate.Term, err)
				continue
			}

			log.Printf("Auto-added new skill: %s (confidence: %.2f)", candidate.Term, candidate.ConfidenceScore)
		}
	}

	return nil
}
