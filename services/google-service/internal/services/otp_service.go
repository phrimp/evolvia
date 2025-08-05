package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"google-service/internal/repository"
	"log"
	"math/big"
	"strconv"
	"time"
)

type OTPService struct {
	redisRepo *repository.RedisRepo
}

type OTPData struct {
	Code      string `json:"code"`
	Email     string `json:"email"`
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	Attempts  int    `json:"attempts"`
}

const (
	OTPExpiryMinutes = 15
	MaxOTPAttempts   = 5
	OTPLength        = 6
)

func NewOTPService(redisRepo *repository.RedisRepo) *OTPService {
	return &OTPService{
		redisRepo: redisRepo,
	}
}

// GenerateOTP generates a new OTP code for email verification
func (o *OTPService) GenerateOTP(userID, email string) (*OTPData, error) {
	// Generate random 6-digit OTP
	otpCode, err := o.generateRandomOTP(OTPLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	now := time.Now()
	otpData := &OTPData{
		Code:      otpCode,
		Email:     email,
		UserID:    userID,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(OTPExpiryMinutes * time.Minute).Unix(),
		Attempts:  0,
	}

	// Store OTP in Redis with expiry
	key := o.getOTPKey(userID)
	_, err = o.redisRepo.SaveStructCached(context.Background(), userID, key, otpData, time.Duration(OTPExpiryMinutes)*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to store OTP in cache: %w", err)
	}

	log.Printf("Generated OTP %s for user %s (email: %s)", otpCode, userID, email)
	return otpData, nil
}

// ValidateOTP validates the provided OTP code
func (o *OTPService) ValidateOTP(userID, otpCode string) (bool, error) {
	key := o.getOTPKey(userID)

	var otpData OTPData
	err := o.redisRepo.GetStructCached(context.Background(), key, userID, &otpData)
	if err != nil {
		log.Printf("OTP not found for user %s: %v", userID, err)
		return false, fmt.Errorf("OTP not found or expired")
	}

	// Check if OTP has expired
	if time.Now().Unix() > otpData.ExpiresAt {
		// Clean up expired OTP
		o.redisRepo.DeleteKey(context.Background(), key)
		return false, fmt.Errorf("OTP has expired")
	}

	// Check attempt limit
	if otpData.Attempts >= MaxOTPAttempts {
		// Clean up OTP after max attempts
		o.redisRepo.DeleteKey(context.Background(), key)
		return false, fmt.Errorf("maximum OTP attempts exceeded")
	}

	// Increment attempts
	otpData.Attempts++

	// Validate OTP code
	if otpData.Code != otpCode {
		// Update attempts in cache
		o.redisRepo.SaveStructCached(context.Background(), userID, key, &otpData, time.Duration(OTPExpiryMinutes)*time.Minute)
		return false, fmt.Errorf("invalid OTP code")
	}

	// OTP is valid - clean up from cache
	o.redisRepo.DeleteKey(context.Background(), key)
	log.Printf("OTP validated successfully for user %s", userID)
	return true, nil
}

// GetOTPStatus returns the current OTP status for a user
func (o *OTPService) GetOTPStatus(userID string) (*OTPData, error) {
	key := o.getOTPKey(userID)

	var otpData OTPData
	err := o.redisRepo.GetStructCached(context.Background(), key, userID, &otpData)
	if err != nil {
		return nil, fmt.Errorf("no active OTP found for user")
	}

	// Check if expired
	if time.Now().Unix() > otpData.ExpiresAt {
		o.redisRepo.DeleteKey(context.Background(), key)
		return nil, fmt.Errorf("OTP has expired")
	}

	return &otpData, nil
}

// ResendOTP generates a new OTP if the current one is expired or doesn't exist
func (o *OTPService) ResendOTP(userID, email string) (*OTPData, error) {
	// Check if there's an existing OTP
	existingOTP, err := o.GetOTPStatus(userID)
	if err == nil && existingOTP != nil {
		// Check if we can resend (e.g., if more than 1 minute has passed)
		timeSinceGeneration := time.Now().Unix() - existingOTP.CreatedAt
		if timeSinceGeneration < 60 { // 1 minute cooldown
			return nil, fmt.Errorf("please wait before requesting a new OTP")
		}
	}

	// Generate new OTP (this will overwrite the existing one)
	return o.GenerateOTP(userID, email)
}

// CleanupExpiredOTPs removes expired OTP records (can be called periodically)
func (o *OTPService) CleanupExpiredOTPs() {
	// This is handled automatically by Redis TTL, but we can implement
	// additional cleanup logic here if needed
	log.Println("OTP cleanup triggered (handled by Redis TTL)")
}

// generateRandomOTP generates a random numeric OTP of specified length
func (o *OTPService) generateRandomOTP(length int) (string, error) {
	otpCode := ""
	for i := 0; i < length; i++ {
		// Generate random number 0-9
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		otpCode += strconv.Itoa(int(num.Int64()))
	}
	return otpCode, nil
}

// getOTPKey generates the Redis key for storing OTP data
func (o *OTPService) getOTPKey(userID string) string {
	return fmt.Sprintf("otp:verification:%s", userID)
}

// GetTimeUntilExpiry returns the time until OTP expiry in minutes
func (o *OTPService) GetTimeUntilExpiry(userID string) (int, error) {
	otpData, err := o.GetOTPStatus(userID)
	if err != nil {
		return 0, err
	}

	timeLeft := otpData.ExpiresAt - time.Now().Unix()
	if timeLeft <= 0 {
		return 0, fmt.Errorf("OTP has expired")
	}

	return int(timeLeft / 60), nil // Return minutes
}
