package service

import (
	"context"
	"fmt"

	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type SessionService struct {
	Repo         *repository.SessionRepository
	QuizRepo     *repository.QuizRepository
	QuestionRepo *repository.QuestionRepository
}

func NewSessionService(
	repo *repository.SessionRepository,
	quizRepo *repository.QuizRepository,
	questionRepo *repository.QuestionRepository,
) *SessionService {
	return &SessionService{
		Repo:         repo,
		QuizRepo:     quizRepo,
		QuestionRepo: questionRepo,
	}
}

func (s *SessionService) GetSession(ctx context.Context, id string) (*models.QuizSession, error) {
	return s.Repo.FindByID(ctx, id)
}

func (s *SessionService) CreateSession(ctx context.Context, session *models.QuizSession) error {
	return s.Repo.Create(ctx, session)
}

func (s *SessionService) UpdateSession(ctx context.Context, id string, update map[string]interface{}) error {
	return s.Repo.Update(ctx, id, update)
}

func (s *SessionService) GetNextQuestion(ctx context.Context, quizID string, answeredIDs []string) (*models.Question, error) {
	// Lấy tất cả question thuộc quiz này
	questions, err := s.QuestionRepo.FindByQuizID(ctx, quizID)
	if err != nil {
		return nil, err
	}
	// Lọc ra các câu hỏi chưa trả về
	var next *models.Question
	for _, q := range questions {
		found := false
		for _, aid := range answeredIDs {
			if q.ID == aid {
				found = true
				break
			}
		}
		if !found {
			next = &q
			break
		}
	}
	if next == nil {
		return nil, fmt.Errorf("no next question")
	}
	return next, nil
}

func (s *SessionService) SubmitSession(ctx context.Context, sessionID, completionType string, finalScore float64) (*models.QuizResult, error) {
	// Cập nhật trạng thái session là đã hoàn thành
	update := map[string]interface{}{
		"status":          "completed",
		"completion_type": completionType,
		"final_score":     finalScore,
	}
	err := s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return nil, err
	}
	// Trả về kết quả (giả sử có QuizResult)
	result := &models.QuizResult{
		SessionID:      sessionID,
		CompletionType: completionType,
		FinalScore:     finalScore,
	}
	return result, nil
}

func (s *SessionService) PauseSession(ctx context.Context, sessionID, reason string) error {
	update := map[string]interface{}{
		"status":       "paused",
		"pause_reason": reason,
	}
	return s.Repo.Update(ctx, sessionID, update)
}
