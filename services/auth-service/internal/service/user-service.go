package service

import (
	"auth_service/internal/events"
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserService struct {
	UserRepo            *repository.UserAuthRepository
	RedisRepo           *repository.RedisRepo
	mu                  *sync.Mutex
	FailedLoginAttempts map[string]*FailedLoginAttempt
	eventPublisher      events.Publisher
}

type FailedLoginAttempt struct {
	failed_at     int64
	failed_number int
}

func NewUserService(eventPublisher events.Publisher) *UserService {
	return &UserService{
		UserRepo:            repository.Repositories_instance.UserAuthRepository,
		mu:                  &sync.Mutex{},
		FailedLoginAttempts: make(map[string]*FailedLoginAttempt),
		RedisRepo:           repository.Repositories_instance.RedisRepository,
		eventPublisher:      eventPublisher,
	}
}

func (us *UserService) Register(ctx context.Context, user *models.UserAuth, profile map[string]string) (bool, error) {
	currentTime := int(time.Now().Unix())

	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}

	user.BasicProfile.DisplayName = profile["fullname"]
	user.CreatedAt = currentTime
	user.UpdatedAt = currentTime
	user.IsActive = true
	user.FailedLoginAttempts = 0

	user_added, err := us.UserRepo.NewUser(ctx, user)
	if err != nil {
		return false, fmt.Errorf("error creating User: %s", err)
	}
	log.Printf("New auth user created: %v", user_added)

	if us.eventPublisher != nil {
		err := us.eventPublisher.PublishUserRegister(
			ctx,
			user.ID.Hex(),
			user.Username,
			user.Email,
			profile,
		)
		if err != nil {
			// Log the error but don't fail the registration
			log.Printf("Warning: Failed to publish user created event: %v", err)
		} else {
			log.Printf("Published user created event for user: %s", user.Username)
		}
	}

	return true, nil
}

func (us *UserService) Login(ctx context.Context, username, password string) (map[string]any, error) {
	if us.RedisRepo.GetInt(ctx, username, "auth-service-lock-user-"+username) != 0 {
		return nil, fmt.Errorf("user is locked")
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
			us.RedisRepo.SaveInt(ctx, username, login_time, 10, "auth-service-lock-user-"+username)
		}
		failed_nums := us.FailedLoginAttempts[username].failed_number
		if failed_nums > 10 {
			log.Printf("User %s, login failed %v time. Locked for %v minute", username, failed_nums, 10)
			us.RedisRepo.SaveInt(ctx, username, login_time, 10, "auth-service-lock-user-"+username)
		}

		us.mu.Lock()
		us.FailedLoginAttempts[username].failed_at = login_time
		us.FailedLoginAttempts[username].failed_number++
		us.mu.Unlock()

		return nil, fmt.Errorf("error finding user with username password: wrong password")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user is not activated")
	}

	login_return := map[string]any{
		"user_id":       user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"basic_profile": user.BasicProfile,
	}

	return login_return, nil
}

func (s *UserService) GetProfile(ctx context.Context, userID bson.ObjectID) (*models.UserWithProfile, error) {
	return nil, nil
}

func (s *UserService) DeactivateUser(ctx context.Context, userID bson.ObjectID) error {
	s.invalidateUserCache(userID.String())
	return nil
}

func (s *UserService) VerifyEmail(ctx context.Context, token string) error {
	return nil
}

func (s *UserService) RequestPasswordReset(ctx context.Context, email string) error {
	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, token string, newPassword string) error {
	return nil
}

func (s *UserService) invalidateUserCache(userID string) {
	s.RedisRepo.DeleteKey(context.Background(), "auth-service-auth-user-"+userID)
	s.RedisRepo.DeleteKey(context.Background(), "user-profile:"+userID)
}

func (us *UserService) CreateDefaultAdminUser(ctx context.Context) error {
	// Check if admin user already exists
	adminUser, err := us.UserRepo.FindByUsername(ctx, "admin")
	if err != nil {
		return fmt.Errorf("error checking for existing admin user: %w", err)
	}

	if adminUser != nil {
		log.Println("Default admin user already exists, skipping creation")
		return nil
	}

	// Check if admin user exists by email
	adminUserByEmail, err := us.UserRepo.FindByEmail(ctx, "admin@evolvia.io")
	if err != nil {
		return fmt.Errorf("error checking for existing admin user by email: %w", err)
	}

	if adminUserByEmail != nil {
		log.Println("Default admin user already exists (by email), skipping creation")
		return nil
	}

	// Create default admin user
	currentTime := int(time.Now().Unix())
	defaultAdminPassword := "Admin123!@#" // You should change this or use env variable

	adminUser = &models.UserAuth{
		ID:                  bson.NewObjectID(),
		Username:            "admin",
		Email:               "admin@evolvia.io",
		PasswordHash:        defaultAdminPassword,
		IsActive:            true,
		IsEmailVerified:     true, // Admin is pre-verified
		FailedLoginAttempts: 0,
		CreatedAt:           currentTime,
		UpdatedAt:           currentTime,
		BasicProfile: models.UserProfile{
			DisplayName: "System Administrator",
		},
	}

	// Create the admin user
	_, err = us.UserRepo.NewUser(ctx, adminUser)
	if err != nil {
		return fmt.Errorf("failed to create default admin user: %w", err)
	}

	log.Printf("Created default admin user with ID: %s", adminUser.ID.Hex())

	// Publish user registration event if publisher is available
	if us.eventPublisher != nil {
		profileData := map[string]string{
			"fullname": "System Administrator",
			"role":     "admin",
		}

		err := us.eventPublisher.PublishUserRegister(
			ctx,
			adminUser.ID.Hex(),
			adminUser.Username,
			adminUser.Email,
			profileData,
		)
		if err != nil {
			log.Printf("Warning: Failed to publish admin user created event: %v", err)
		} else {
			log.Printf("Published user created event for admin user")
		}
	}

	log.Println("Successfully created default admin user")
	log.Printf("Admin credentials - Username: %s, Email: %s, Password: %s",
		adminUser.Username, adminUser.Email, defaultAdminPassword)
	log.Println("IMPORTANT: Please change the default admin password after first login!")

	return nil
}
