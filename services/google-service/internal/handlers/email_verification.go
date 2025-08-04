package handlers

import (
	"fmt"
	"google-service/internal/event"
	"google-service/internal/services"
	"log"

	"github.com/gofiber/fiber/v3"
)

type EmailVerificationHandler struct {
	otpService     *services.OTPService
	emailService   *services.EmailService
	eventPublisher *event.EventPublisher
}

type VerifyEmailRequest struct {
	UserID  string `json:"user_id" validate:"required"`
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

type ResendOTPRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Email  string `json:"email" validate:"required,email"`
}

type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

type ResendOTPResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ExpiryTime string `json:"expiry_time,omitempty"`
}

type OTPStatusResponse struct {
	Success       bool   `json:"success"`
	HasActiveOTP  bool   `json:"has_active_otp"`
	TimeRemaining int    `json:"time_remaining_minutes,omitempty"`
	AttemptsLeft  int    `json:"attempts_left,omitempty"`
	Message       string `json:"message,omitempty"`
}

func NewEmailVerificationHandler(
	otpService *services.OTPService,
	emailService *services.EmailService,
	eventPublisher *event.EventPublisher,
) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		otpService:     otpService,
		emailService:   emailService,
		eventPublisher: eventPublisher,
	}
}

func (h *EmailVerificationHandler) RegisterRoutes(app *fiber.App) {
	apiGroup := app.Group("public/google")
	emailGroup := apiGroup.Group("/email")

	emailGroup.Post("/verify", h.VerifyEmail)
	emailGroup.Post("/resend-otp", h.ResendOTP)
	emailGroup.Get("/otp-status/:user_id", h.GetOTPStatus)
}

// VerifyEmail handles email verification with OTP
func (h *EmailVerificationHandler) VerifyEmail(c fiber.Ctx) error {
	var req VerifyEmailRequest

	if err := c.Bind().Body(&req); err != nil {
		log.Printf("Failed to parse verify email request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(VerifyEmailResponse{
			Success: false,
			Message: "Invalid request format",
		})
	}

	// Validate required fields
	if req.UserID == "" || req.OTPCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(VerifyEmailResponse{
			Success: false,
			Message: "user_id and otp_code are required",
		})
	}

	// Validate OTP code format (6 digits)
	if len(req.OTPCode) != 6 {
		return c.Status(fiber.StatusBadRequest).JSON(VerifyEmailResponse{
			Success: false,
			Message: "OTP code must be 6 digits",
		})
	}

	// Validate OTP
	isValid, err := h.otpService.ValidateOTP(req.UserID, req.OTPCode)
	if err != nil {
		log.Printf("OTP validation failed for user %s: %v", req.UserID, err)

		// Return specific error messages for better UX
		errorMessage := "Invalid or expired OTP code"
		if err.Error() == "maximum OTP attempts exceeded" {
			errorMessage = "Maximum OTP attempts exceeded. Please request a new OTP"
		} else if err.Error() == "OTP has expired" {
			errorMessage = "OTP has expired. Please request a new OTP"
		} else if err.Error() == "invalid OTP code" {
			errorMessage = "Invalid OTP code. Please check and try again"
		}

		return c.Status(fiber.StatusBadRequest).JSON(VerifyEmailResponse{
			Success: false,
			Message: errorMessage,
		})
	}

	if !isValid {
		return c.Status(fiber.StatusBadRequest).JSON(VerifyEmailResponse{
			Success: false,
			Message: "Invalid OTP code",
		})
	}

	// Get OTP data to retrieve email before it's deleted
	otpData, err := h.otpService.GetOTPStatus(req.UserID)
	if err != nil {
		// OTP might have been deleted after validation, try to proceed with event publishing
		log.Printf("Warning: Could not retrieve OTP data for user %s: %v", req.UserID, err)
	}

	// OTP is valid - publish email verification success event
	var email string
	if otpData != nil {
		email = otpData.Email
	}

	err = h.eventPublisher.PublishEmailVerificationSuccess(c.Context(), req.UserID, email)
	if err != nil {
		log.Printf("Failed to publish email verification success event for user %s: %v", req.UserID, err)
		// Don't fail the request if event publishing fails
	} else {
		log.Printf("Email verification success event published for user %s", req.UserID)
	}

	// Send welcome email if we have the email
	if email != "" {
		welcomeData := services.EmailData{
			Name:      "User", // Could be enhanced with actual user name
			Email:     email,
			VerifyURL: "https://your-frontend-url.com/dashboard", // Could be configurable
		}

		err = h.emailService.SendEmailWithTemplate("welcome", welcomeData, []string{email})
		if err != nil {
			log.Printf("Failed to send welcome email to %s: %v", email, err)
			// Don't fail the verification if welcome email fails
		} else {
			log.Printf("Welcome email sent successfully to %s", email)
		}
	}

	return c.Status(fiber.StatusOK).JSON(VerifyEmailResponse{
		Success: true,
		Message: "Email verified successfully",
		UserID:  req.UserID,
	})
}

// ResendOTP handles resending OTP for email verification
func (h *EmailVerificationHandler) ResendOTP(c fiber.Ctx) error {
	var req ResendOTPRequest

	if err := c.Bind().Body(&req); err != nil {
		log.Printf("Failed to parse resend OTP request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(ResendOTPResponse{
			Success: false,
			Message: "Invalid request format",
		})
	}

	// Validate required fields
	if req.UserID == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ResendOTPResponse{
			Success: false,
			Message: "user_id and email are required",
		})
	}

	// Generate new OTP
	otpData, err := h.otpService.ResendOTP(req.UserID, req.Email)
	if err != nil {
		log.Printf("Failed to resend OTP for user %s: %v", req.UserID, err)

		errorMessage := "Failed to generate new OTP"
		if err.Error() == "please wait before requesting a new OTP" {
			errorMessage = "Please wait at least 1 minute before requesting a new OTP"
		}

		return c.Status(fiber.StatusTooManyRequests).JSON(ResendOTPResponse{
			Success: false,
			Message: errorMessage,
		})
	}

	// Prepare email data
	emailData := services.EmailData{
		Name:       "User", // Could be enhanced with actual user name
		Email:      req.Email,
		OTPCode:    otpData.Code,
		ExpiryTime: fmt.Sprintf("%d minutes", (otpData.ExpiresAt-otpData.CreatedAt)/60),
		VerifyURL:  fmt.Sprintf("https://your-frontend-url.com/verify-email?user_id=%s&otp=%s", req.UserID, otpData.Code),
	}

	// Send verification email
	err = h.emailService.SendEmailWithTemplate("email_verification", emailData, []string{req.Email})
	if err != nil {
		log.Printf("Failed to send verification email to %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(ResendOTPResponse{
			Success: false,
			Message: "OTP generated but failed to send email. Please try again",
		})
	}

	log.Printf("OTP resent successfully to %s for user %s", req.Email, req.UserID)

	return c.Status(fiber.StatusOK).JSON(ResendOTPResponse{
		Success:    true,
		Message:    "OTP sent successfully",
		ExpiryTime: fmt.Sprintf("%d minutes", (otpData.ExpiresAt-otpData.CreatedAt)/60),
	})
}

// GetOTPStatus returns the current OTP status for a user
func (h *EmailVerificationHandler) GetOTPStatus(c fiber.Ctx) error {
	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(OTPStatusResponse{
			Success: false,
			Message: "user_id is required",
		})
	}

	// Get OTP status
	otpData, err := h.otpService.GetOTPStatus(userID)
	if err != nil {
		// No active OTP found
		return c.Status(fiber.StatusOK).JSON(OTPStatusResponse{
			Success:      true,
			HasActiveOTP: false,
			Message:      "No active OTP found",
		})
	}

	// Calculate time remaining
	timeRemaining, err := h.otpService.GetTimeUntilExpiry(userID)
	if err != nil {
		return c.Status(fiber.StatusOK).JSON(OTPStatusResponse{
			Success:      true,
			HasActiveOTP: false,
			Message:      "OTP has expired",
		})
	}

	// Calculate attempts left
	attemptsLeft := services.MaxOTPAttempts - otpData.Attempts
	if attemptsLeft < 0 {
		attemptsLeft = 0
	}

	return c.Status(fiber.StatusOK).JSON(OTPStatusResponse{
		Success:       true,
		HasActiveOTP:  true,
		TimeRemaining: timeRemaining,
		AttemptsLeft:  attemptsLeft,
		Message:       "Active OTP found",
	})
}

