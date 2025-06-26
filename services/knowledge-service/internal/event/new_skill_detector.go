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

// ImprovedSkillDiscovery with better detection for educational and technical content
type ImprovedSkillDiscovery struct {
	skillService        *services.SkillService
	confidenceThreshold float64
	minFrequency        int
	techKeywords        []string
	skillPatterns       []*regexp.Regexp
	educationalPatterns []*regexp.Regexp
}

func NewImprovedSkillDiscoveryService() *ImprovedSkillDiscovery {
	return &ImprovedSkillDiscovery{
		confidenceThreshold: 0.5, // Lowered threshold
		minFrequency:        2,   // Lowered minimum frequency
		techKeywords: []string{
			"programming", "framework", "library", "tool", "platform",
			"language", "database", "api", "sdk", "ide", "version control",
			"testing", "deployment", "cloud", "devops", "agile", "methodology",
			"algorithm", "structure", "implementation", "class", "method",
			"exception", "array", "linked", "stack", "queue", "tree", "graph",
		},
		skillPatterns: []*regexp.Regexp{
			// Programming languages and technologies
			regexp.MustCompile(`(?i)\b(Java|Python|JavaScript|C\+\+|C#|Go|Rust|Swift|Kotlin|Scala|PHP|Ruby|TypeScript)\b`),
			// Data structures and algorithms
			regexp.MustCompile(`(?i)\b(Stack|Queue|Array|LinkedList|ArrayList|Tree|Graph|HashMap|HashSet|Binary Tree|AVL Tree|Red-Black Tree|Heap|Priority Queue)\b`),
			// Programming concepts
			regexp.MustCompile(`(?i)\b(Object[-\s]?Oriented Programming|OOP|Recursion|Dynamic Programming|Greedy Algorithm|Sorting|Searching|Big O|Time Complexity|Space Complexity)\b`),
			// Frameworks and libraries
			regexp.MustCompile(`(?i)\b(Spring|Hibernate|JUnit|Maven|Gradle|React|Angular|Vue|Django|Flask|Express|Node\.js)\b`),
			// Tools and platforms
			regexp.MustCompile(`(?i)\b(Git|GitHub|GitLab|Docker|Kubernetes|Jenkins|IntelliJ|Eclipse|Visual Studio|VS Code)\b`),
			// Database technologies
			regexp.MustCompile(`(?i)\b(MySQL|PostgreSQL|MongoDB|Redis|Oracle|SQL Server|SQLite|Cassandra|DynamoDB)\b`),
			// Web technologies
			regexp.MustCompile(`(?i)\b(HTML|CSS|XML|JSON|REST|SOAP|HTTP|HTTPS|WebSocket|GraphQL)\b`),
		},
		educationalPatterns: []*regexp.Regexp{
			// Course topics and subjects
			regexp.MustCompile(`(?i)Data Structures and Algorithms in\s+(\w+)`),
			regexp.MustCompile(`(?i)(\w+)[-\s]?based\s+(?:implementation|stack|queue|tree)`),
			regexp.MustCompile(`(?i)(?:class|interface|abstract)\s+(\w+)`),
			regexp.MustCompile(`(?i)(\w+)\.util\.(\w+)`),     // Java package references
			regexp.MustCompile(`(?i)import\s+[\w.]+\.(\w+)`), // Import statements
		},
	}
}

// DiscoverNewSkills with improved detection for the provided content
func (isd *ImprovedSkillDiscovery) DiscoverNewSkills(ctx context.Context, content string, source string) ([]*SkillCandidate, error) {
	log.Printf("Starting skill discovery on content length: %d", len(content))

	candidates := make(map[string]*SkillCandidate)

	// Technique 1: Enhanced pattern-based extraction
	patternCandidates := isd.extractByEnhancedPatterns(content, source)
	isd.mergeCandidates(candidates, patternCandidates, source)
	log.Printf("Pattern extraction found %d candidates", len(patternCandidates))

	// Technique 2: Technical term extraction (improved)
	techCandidates := isd.extractTechnicalTerms(content, source)
	isd.mergeCandidates(candidates, techCandidates, source)
	log.Printf("Technical term extraction found %d candidates", len(techCandidates))

	// Technique 3: Educational content analysis
	eduCandidates := isd.extractEducationalTerms(content, source)
	isd.mergeCandidates(candidates, eduCandidates, source)
	log.Printf("Educational extraction found %d candidates", len(eduCandidates))

	// Technique 4: Programming concept extraction
	conceptCandidates := isd.extractProgrammingConcepts(content, source)
	isd.mergeCandidates(candidates, conceptCandidates, source)
	log.Printf("Programming concept extraction found %d candidates", len(conceptCandidates))

	log.Printf("Total unique candidates before filtering: %d", len(candidates))

	// Filter and score candidates
	filteredCandidates := isd.filterAndScore(candidates, content)
	log.Printf("Candidates after filtering: %d", len(filteredCandidates))

	// Check against existing skills
	newCandidates := []*SkillCandidate{}
	for _, candidate := range filteredCandidates {
		if !isd.isExistingSkill(ctx, candidate.Term) {
			newCandidates = append(newCandidates, candidate)
		}
	}

	log.Printf("New skill candidates (not in database): %d", len(newCandidates))

	// Sort by confidence score
	sort.Slice(newCandidates, func(i, j int) bool {
		return newCandidates[i].ConfidenceScore > newCandidates[j].ConfidenceScore
	})

	// Log top candidates for debugging
	for i, candidate := range newCandidates {
		if i < 10 { // Log top 10
			log.Printf("Candidate #%d: %s (confidence: %.2f, frequency: %d)",
				i+1, candidate.Term, candidate.ConfidenceScore, candidate.Frequency)
		}
	}

	return newCandidates, nil
}

// extractByEnhancedPatterns uses improved regex patterns
func (isd *ImprovedSkillDiscovery) extractByEnhancedPatterns(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	// Combine all patterns (skill + educational)
	allPatterns := append(isd.skillPatterns, isd.educationalPatterns...)

	for _, pattern := range allPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			for i := 1; i < len(match); i++ { // Skip full match, process captured groups
				if match[i] != "" {
					term := strings.TrimSpace(match[i])
					if isd.isValidSkillTerm(term) {
						if existing, exists := candidates[term]; exists {
							existing.Frequency++
							existing.Contexts = append(existing.Contexts, match[0])
						} else {
							candidates[term] = &SkillCandidate{
								Term:      term,
								Frequency: 1,
								Contexts:  []string{match[0]},
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

	return candidates
}

// extractTechnicalTerms with improved detection
func (isd *ImprovedSkillDiscovery) extractTechnicalTerms(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	// Look for capitalized technical terms
	technicalTermPattern := regexp.MustCompile(`\b[A-Z][a-zA-Z]*(?:[A-Z][a-zA-Z]*)*\b`)
	matches := technicalTermPattern.FindAllString(content, -1)

	for _, match := range matches {
		if isd.isTechnicalTerm(match) {
			if existing, exists := candidates[match]; exists {
				existing.Frequency++
			} else {
				candidates[match] = &SkillCandidate{
					Term:      match,
					Frequency: 1,
					FirstSeen: time.Now(),
					LastSeen:  time.Now(),
					Sources:   map[string]int{source: 1},
				}
			}
		}
	}

	return candidates
}

// extractEducationalTerms specifically for educational content
func (isd *ImprovedSkillDiscovery) extractEducationalTerms(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	// Educational indicators
	eduIndicators := []string{
		"implementation", "algorithm", "data structure", "abstract data type",
		"class", "method", "function", "operation", "exception",
	}

	// Look for terms near educational indicators
	sentences := strings.Split(content, ".")
	for _, sentence := range sentences {
		sentence = strings.ToLower(strings.TrimSpace(sentence))

		for _, indicator := range eduIndicators {
			if strings.Contains(sentence, indicator) {
				// Extract potential skills from this sentence
				words := strings.Fields(sentence)
				for _, word := range words {
					word = strings.TrimSpace(word)
					if isd.isEducationalSkillTerm(word) {
						term := strings.Title(word)
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

	return candidates
}

// extractProgrammingConcepts for specific programming terms
func (isd *ImprovedSkillDiscovery) extractProgrammingConcepts(content, source string) map[string]*SkillCandidate {
	candidates := make(map[string]*SkillCandidate)

	// Programming concepts that should be treated as skills
	programmingConcepts := []string{
		"Stack", "Queue", "Array", "LinkedList", "ArrayList", "Vector",
		"LIFO", "FIFO", "Big O", "Time Complexity", "Space Complexity",
		"Recursion", "Iteration", "Exception Handling", "Memory Management",
		"Object Oriented Programming", "OOP", "Inheritance", "Polymorphism",
		"Encapsulation", "Abstraction", "Interface", "Abstract Class",
		"Generics", "Collections", "HashMap", "HashSet", "TreeMap", "TreeSet",
		"Binary Tree", "Binary Search", "Linear Search", "Bubble Sort",
		"Quick Sort", "Merge Sort", "Heap Sort", "Insertion Sort",
	}

	contentLower := strings.ToLower(content)

	for _, concept := range programmingConcepts {
		conceptLower := strings.ToLower(concept)
		count := strings.Count(contentLower, conceptLower)

		if count > 0 {
			// Find actual contexts where this concept appears
			pattern := regexp.MustCompile(`(?i)([^.]*` + regexp.QuoteMeta(conceptLower) + `[^.]*)`)
			contexts := pattern.FindAllString(content, -1)

			candidates[concept] = &SkillCandidate{
				Term:      concept,
				Frequency: count,
				Contexts:  contexts,
				FirstSeen: time.Now(),
				LastSeen:  time.Now(),
				Sources:   map[string]int{source: count},
			}
		}
	}

	return candidates
}

// isTechnicalTerm checks if a term is likely technical
func (isd *ImprovedSkillDiscovery) isTechnicalTerm(term string) bool {
	if len(term) < 3 || len(term) > 30 {
		return false
	}

	// Skip common English words
	commonWords := []string{
		"The", "And", "For", "Are", "But", "Not", "You", "All", "Can", "Had",
		"Her", "Was", "One", "Our", "Out", "Day", "Get", "Has", "Him", "His",
		"How", "Its", "May", "New", "Now", "Old", "See", "Two", "Who", "Boy",
		"Did", "Man", "Men", "Put", "Say", "She", "Too", "Use", "What", "When",
		"Where", "Why", "Will", "With", "Work", "Your", "About", "After", "Again",
		"Before", "Being", "Below", "Between", "During", "Each", "Few", "From",
		"Further", "Here", "How", "Into", "More", "Most", "Other", "Over", "Same",
		"Some", "Such", "Than", "That", "Their", "Them", "These", "They", "This",
		"Those", "Through", "Under", "Until", "Very", "What", "When", "Where",
		"Which", "While", "With", "Would", "There", "Could", "Should", "Class",
		"Public", "Private", "Protected", "Static", "Final", "Abstract", "Return",
		"Throw", "Throws", "Import", "Package", "Extends", "Implements", "Super",
		"This", "Null", "True", "False", "If", "Else", "While", "For", "Do",
		"Switch", "Case", "Break", "Continue", "Try", "Catch", "Finally",
		"Implementation", "Algorithm", "Operation", "Element", "Method", "Function",
		"Variable", "Parameter", "Argument", "Value", "Type", "Object", "Instance",
		"Reference", "Pointer", "Memory", "Size", "Length", "Index", "Position",
		"First", "Last", "Next", "Previous", "Current", "Empty", "Full",
	}

	for _, common := range commonWords {
		if strings.EqualFold(term, common) {
			return false
		}
	}

	// Check if it looks like a technical term
	technicalPatterns := []string{
		"Exception", "Error", "List", "Set", "Map", "Tree", "Node", "Stack",
		"Queue", "Array", "Buffer", "Cache", "Pool", "Factory", "Builder",
		"Handler", "Manager", "Service", "Controller", "Repository", "DAO",
	}

	for _, pattern := range technicalPatterns {
		if strings.Contains(term, pattern) {
			return true
		}
	}

	// If it's all caps and longer than 2 characters, likely an acronym
	if len(term) > 2 && strings.ToUpper(term) == term {
		return true
	}

	return true // Default to including it
}

// isEducationalSkillTerm checks for educational/academic skill terms
func (isd *ImprovedSkillDiscovery) isEducationalSkillTerm(term string) bool {
	if len(term) < 3 {
		return false
	}

	educationalSkillTerms := []string{
		"java", "python", "javascript", "stack", "queue", "array", "list",
		"tree", "graph", "algorithm", "sorting", "searching", "recursion",
		"iteration", "complexity", "optimization", "debugging", "testing",
		"validation", "implementation", "design", "analysis", "structure",
	}

	termLower := strings.ToLower(term)
	for _, skillTerm := range educationalSkillTerms {
		if termLower == skillTerm {
			return true
		}
	}

	return false
}

// Enhanced validation
func (isd *ImprovedSkillDiscovery) isValidSkillTerm(term string) bool {
	term = strings.TrimSpace(term)
	if len(term) < 2 || len(term) > 50 {
		return false
	}

	// Skip numbers and very short terms
	if regexp.MustCompile(`^\d+$`).MatchString(term) {
		return false
	}

	// Skip single characters
	if len(term) == 1 {
		return false
	}

	return true
}

// Enhanced confidence calculation
func (isd *ImprovedSkillDiscovery) calculateConfidence(candidate *SkillCandidate, content string) float64 {
	score := 0.0

	// Base score from frequency (more generous)
	score += float64(candidate.Frequency) * 0.15

	// Bonus for appearing in multiple sources
	if len(candidate.Sources) > 1 {
		score += 0.2
	}

	// Bonus for technical context
	techContextBonus := 0.0
	for _, context := range candidate.Contexts {
		contextLower := strings.ToLower(context)
		for _, keyword := range isd.techKeywords {
			if strings.Contains(contextLower, keyword) {
				techContextBonus += 0.1
				break // Only count once per context
			}
		}
	}
	score += techContextBonus

	// Bonus for proper capitalization
	if isd.isProperlyCapitalized(candidate.Term) {
		score += 0.15
	}

	// Bonus for known programming terms
	programmingTerms := []string{
		"java", "stack", "queue", "array", "list", "tree", "algorithm",
		"exception", "class", "method", "implementation", "structure",
	}
	termLower := strings.ToLower(candidate.Term)
	for _, progTerm := range programmingTerms {
		if strings.Contains(termLower, progTerm) {
			score += 0.25
			break
		}
	}

	// Bonus for appearing in educational context
	educationalIndicators := []string{
		"data structures", "algorithms", "programming", "implementation",
		"class", "method", "exception", "abstract data type",
	}
	contentLower := strings.ToLower(content)
	for _, indicator := range educationalIndicators {
		if strings.Contains(contentLower, indicator) {
			score += 0.1
			break
		}
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// Rest of the helper methods remain the same but with updated logic...
func (isd *ImprovedSkillDiscovery) mergeCandidates(target map[string]*SkillCandidate, source map[string]*SkillCandidate, sourceName string) {
	for term, candidate := range source {
		if existing, exists := target[term]; exists {
			existing.Frequency += candidate.Frequency
			existing.Contexts = append(existing.Contexts, candidate.Contexts...)
			if sourceName != "" {
				existing.Sources[sourceName] += candidate.Frequency
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

func (isd *ImprovedSkillDiscovery) filterAndScore(candidates map[string]*SkillCandidate, content string) []*SkillCandidate {
	var filtered []*SkillCandidate

	for _, candidate := range candidates {
		// Apply minimum frequency filter (lowered)
		if candidate.Frequency < isd.minFrequency {
			continue
		}

		// Calculate confidence score
		confidence := isd.calculateConfidence(candidate, content)
		candidate.ConfidenceScore = confidence

		// Apply confidence threshold (lowered)
		if confidence >= isd.confidenceThreshold {
			candidate.Category = isd.categorizeSkill(candidate.Term)
			candidate.SuggestedLevel = isd.suggestSkillLevel(candidate, content)
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

// Simplified existence check for testing
func (isd *ImprovedSkillDiscovery) isExistingSkill(ctx context.Context, term string) bool {
	// For now, assume none exist so we can see what gets detected
	// In real implementation, check against your skill database
	return false
}

func (isd *ImprovedSkillDiscovery) isProperlyCapitalized(term string) bool {
	if len(term) == 0 {
		return false
	}
	return term[0] >= 'A' && term[0] <= 'Z'
}

func (isd *ImprovedSkillDiscovery) categorizeSkill(term string) string {
	categories := map[string][]string{
		"Programming Languages": {"java", "python", "javascript", "c++", "go", "rust", "swift"},
		"Data Structures":       {"stack", "queue", "array", "list", "tree", "graph", "heap"},
		"Algorithms":            {"sorting", "searching", "recursion", "algorithm", "complexity"},
		"Programming Concepts":  {"oop", "inheritance", "polymorphism", "exception", "class", "method"},
		"Tools":                 {"git", "jenkins", "jira", "eclipse", "intellij"},
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

func (isd *ImprovedSkillDiscovery) suggestSkillLevel(candidate *SkillCandidate, content string) models.SkillLevel {
	return models.SkillLevelBeginner // Default for educational content
}

func (nsd *ImprovedSkillDiscovery) AutoAddNewSkills(ctx context.Context, candidates []*SkillCandidate, autoAddThreshold float64) error {
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
