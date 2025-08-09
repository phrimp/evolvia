package models

type Option struct {
	ID   string `bson:"id" json:"id"`
	Text string `bson:"text" json:"text"`
}

type Question struct {
	ID                   string   `bson:"_id,omitempty" json:"id"`
	Content              string   `bson:"content" json:"content"`
	Type                 string   `bson:"type" json:"type"`
	Options              []Option `bson:"options" json:"options"`
	CorrectAnswer        string   `bson:"correct_answer" json:"correct_answer"`
	Explanation          string   `bson:"explanation" json:"explanation"`
	SkillID              string   `bson:"skill_id" json:"skill_id"`
	DifficultyLevel      string   `bson:"difficulty_level" json:"difficulty_level"`
	BloomLevel           string   `bson:"bloom_level" json:"bloom_level"`
	Points               int      `bson:"points" json:"points"`
	EstimatedTimeSeconds int      `bson:"estimated_time_seconds" json:"estimated_time_seconds"`
	TopicTags            []string `bson:"topic_tags" json:"topic_tags"`
	QuestionPoolID       string   `bson:"question_pool_id" json:"question_pool_id"`
	// New Bloom scoring fields
	BloomScore         int            `bson:"bloom_score" json:"bloom_score"`
	BloomScoresByStage map[string]int `bson:"bloom_scores_by_stage" json:"bloom_scores_by_stage"`
}

// BloomBaseScores defines base scores for each Bloom taxonomy level
var BloomBaseScores = map[string]int{
	"remember":   10,
	"understand": 15,
	"apply":      20,
	"analyze":    25,
	"evaluate":   30,
	"create":     35,
}

// StageMultipliers defines score multipliers for difficulty stages
var StageMultipliers = map[string]float64{
	"easy":   1.0,
	"medium": 1.2,
	"hard":   1.5,
}

// CalculateBloomScore calculates and sets the Bloom score based on Bloom level
func (q *Question) CalculateBloomScore() {
	if baseScore, exists := BloomBaseScores[q.BloomLevel]; exists {
		q.BloomScore = baseScore
	} else {
		q.BloomScore = 10 // Default fallback
	}
}

// CalculateBloomScoresByStage calculates scores for all difficulty stages
func (q *Question) CalculateBloomScoresByStage() {
	q.CalculateBloomScore() // Ensure base score is calculated

	q.BloomScoresByStage = make(map[string]int)
	for stage, multiplier := range StageMultipliers {
		q.BloomScoresByStage[stage] = int(float64(q.BloomScore) * multiplier)
	}
}

// GetScoreForStage returns the appropriate score for a given stage
func (q *Question) GetScoreForStage(stage string) int {
	if q.BloomScoresByStage == nil {
		q.CalculateBloomScoresByStage()
	}

	if stageScore, exists := q.BloomScoresByStage[stage]; exists {
		return stageScore
	}

	// Fallback to base score
	return q.BloomScore
}

// EnsureBloomScores ensures both bloom score fields are populated
func (q *Question) EnsureBloomScores() {
	if q.BloomScore == 0 {
		q.CalculateBloomScore()
	}
	if len(q.BloomScoresByStage) == 0 {
		q.CalculateBloomScoresByStage()
	}
}
