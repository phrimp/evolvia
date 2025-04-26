package service

import (
	"auth_service/internal/database/mongo"
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService struct {
	UserRepo            *repository.UserAuthRepository
	mu                  *sync.Mutex
	FailedLoginAttempts map[string]*FailedLoginAttempt
}

type FailedLoginAttempt struct {
	failed_at     int64
	failed_number int
}

func NewUserService() *UserService {
	return &UserService{
		UserRepo:            repository.NewUserAuthRepository(mongo.Mongo_Database),
		mu:                  &sync.Mutex{},
		FailedLoginAttempts: make(map[string]*FailedLoginAttempt),
	}
}

func (us *UserService) Register(ctx context.Context, user *models.UserAuth, profile map[string]string) (bool, error) {
	currentTime := int(time.Now().Unix())

	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
	}

	user.CreatedAt = currentTime
	user.UpdatedAt = currentTime
	user.IsActive = true
	user.FailedLoginAttempts = 0

	user_added, err := us.UserRepo.NewUser(ctx, user)
	if err != nil {
		return false, fmt.Errorf("error creating User: %s", err)
	}
	log.Printf("New auth user created: %v", user_added)

	// _, err := CreateUserProfile(profile)

	return true, nil
}

func (us *UserService) Login(ctx context.Context, username, password string) (map[string]string, error) {
	if repository.RedisRepository.GetInt(ctx, username, "auth-service-lock-user-"+username) != 0 {
		return nil, fmt.Errorf("user locked")
	}
	user, err := us.UserRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("error finding username: %s", err)
	}
	isPassword := us.UserRepo.VerifyPassword(user, password)
	login_time := time.Now().Local().UnixMilli()

	if !isPassword {
		if us.FailedLoginAttempts[username] == nil {
			us.FailedLoginAttempts[username] = &FailedLoginAttempt{}
		}
		last_failed_login_attempt := us.FailedLoginAttempts[username].failed_at
		if login_time-last_failed_login_attempt < 1000 {
			log.Printf("WARN: Suspicious activity detect for user: %s. Instant locked activated", username)
			repository.RedisRepository.SaveInt(ctx, username, login_time, 10, "auth-service-lock-user-"+username)
		}
		failed_nums := us.FailedLoginAttempts[username].failed_number
		if failed_nums > 10 {
			log.Printf("User %s, login failed %v time. Locked for %v minute", username, failed_nums, 10)
			repository.RedisRepository.SaveInt(ctx, username, login_time, 10, "auth-service-lock-user-"+username)
		}

		us.mu.Lock()
		us.FailedLoginAttempts[username].failed_at = login_time
		us.FailedLoginAttempts[username].failed_number++
		us.mu.Unlock()

		return nil, fmt.Errorf("error finding user with username password: %s", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user is not activated")
	}
	// session, err := GenerateSession()

	return make(map[string]string), nil
}
