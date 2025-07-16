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
}
