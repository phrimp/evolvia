package service

import (
	"auth_service/internal/events"
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
	"log"
	"strings"
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

// ListAllUsers returns paginated list of all users for admin purposes
func (us *UserService) ListAllUsers(ctx context.Context, page, limit int) ([]*models.UserAuth, error) {
	users, err := us.UserRepo.FindAll(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %w", err)
	}

	return users, nil
}

// GetUserByID returns a user by their ID for admin purposes
func (us *UserService) GetUserByID(ctx context.Context, userID bson.ObjectID) (*models.UserAuth, error) {
	user, err := us.UserRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return user, nil
}

// UpdateUserByAdmin updates user information by admin
func (us *UserService) UpdateUserByAdmin(ctx context.Context, userID bson.ObjectID, updateRequest interface{}) (*models.UserAuth, error) {
	// First get the current user
	user, err := us.UserRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Type assertion to get the update fields
	if updateData, ok := updateRequest.(*struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		IsActive        *bool  `json:"isActive"`
		IsEmailVerified *bool  `json:"isEmailVerified"`
	}); ok {
		currentTime := int(time.Now().Unix())

		// Update fields if provided
		if updateData.Username != "" {
			user.Username = updateData.Username
		}
		if updateData.Email != "" {
			user.Email = updateData.Email
		}
		if updateData.IsActive != nil {
			user.IsActive = *updateData.IsActive
		}
		if updateData.IsEmailVerified != nil {
			user.IsEmailVerified = *updateData.IsEmailVerified
		}

		user.UpdatedAt = currentTime

		// Update in repository
		err = us.UserRepo.Update(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		// Invalidate cache
		us.invalidateUserCache(userID.String())

		return user, nil
	}

	return nil, fmt.Errorf("invalid update request format")
}

// ActivateUser activates a user account
func (us *UserService) ActivateUser(ctx context.Context, userID bson.ObjectID) error {
	user, err := us.UserRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.IsActive = true
	user.UpdatedAt = int(time.Now().Unix())

	err = us.UserRepo.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to activate user: %w", err)
	}

	// Invalidate cache
	us.invalidateUserCache(userID.String())

	return nil
}

// SearchUsers searches for users based on criteria
func (us *UserService) SearchUsers(ctx context.Context, username, email, isActive string, page, limit int) ([]*models.UserAuth, error) {
	// For now, we'll get a larger set of users and filter them
	// In a production system, you'd want to implement this filtering at the database level
	maxResults := 1000 // Get up to 1000 users to search through
	allUsers, err := us.UserRepo.FindAll(ctx, 1, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %w", err)
	}

	// Filter users based on search criteria
	var filteredUsers []*models.UserAuth
	for _, user := range allUsers {
		match := true

		if username != "" && !containsIgnoreCase(user.Username, username) {
			match = false
		}
		if email != "" && !containsIgnoreCase(user.Email, email) {
			match = false
		}
		if isActive != "" {
			activeFilter := isActive == "true"
			if user.IsActive != activeFilter {
				match = false
			}
		}

		if match {
			filteredUsers = append(filteredUsers, user)
		}
	}

	// Apply pagination to filtered results
	start := (page - 1) * limit
	end := start + limit

	if start >= len(filteredUsers) {
		return []*models.UserAuth{}, nil
	}

	if end > len(filteredUsers) {
		end = len(filteredUsers)
	}

	return filteredUsers[start:end], nil
}

// Helper function for case-insensitive string contains
func containsIgnoreCase(source, substr string) bool {
	if substr == "" {
		return true
	}
	if len(source) < len(substr) {
		return false
	}

	sourceLower := strings.ToLower(source)
	substrLower := strings.ToLower(substr)

	return strings.Contains(sourceLower, substrLower)
}
