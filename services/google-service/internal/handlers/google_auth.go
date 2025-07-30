package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"google-service/internal/config"
	"google-service/internal/event"
	"google-service/internal/models"
	"google-service/internal/repository"
	"google-service/internal/services"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
)

var is_state map[string]string = make(map[string]string)

type AuthHandler struct {
	oauthService   *services.GoogleOAuthService
	redisRepo      *repository.RedisRepo
	eventPublisher *event.EventPublisher
	FE_Address     string
}

func NewAuthHandler(google_config *config.GoogleOAuthConfig, address string, redisRepo *repository.RedisRepo, eventPublisher *event.EventPublisher) *AuthHandler {
	return &AuthHandler{
		oauthService:   services.NewGoogleOAuthService(google_config),
		FE_Address:     address,
		redisRepo:      redisRepo,
		eventPublisher: eventPublisher,
	}
}

func (h *AuthHandler) RegisterRoutes(app *fiber.App) {
	publicGroup := app.Group("public/google")
	authGroup := publicGroup.Group("/auth")
	authGroup.Get("/", h.HandleGoogleLogin)
	authGroup.Get("/callback", h.HandleGoogleCallback)
}

func (h *AuthHandler) HandleGoogleLogin(c fiber.Ctx) error {
	state := generateRandomState()
	state = state[:len(state)-1]

	if len(state) < 32 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate secure state",
		})
	}

	is_state[state] = state
	_, err := h.redisRepo.SaveStructCached(c.Context(), "", "google-auth-state:", state, 1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to cached state",
		})
	}

	log.Println("Set state:", state)

	url := h.oauthService.GetAuthURL(state)
	return c.Redirect().To(url)
}

func (h *AuthHandler) HandleGoogleCallback(c fiber.Ctx) error {
	state, ok := is_state[c.Query("state")]
	log.Printf("check state: %s with state %s", state, c.Query("state"))
	if !ok || state == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid state",
		})
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code is missing",
		})
	}

	token, err := h.oauthService.Exchange(c.Context(), code)
	if err != nil {
		log.Printf("Token exchange error: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to exchange token",
			"details": err.Error(),
		})
	}

	userInfo, err := h.oauthService.GetUserInfo(token)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user info",
		})
	}

	// Store user token for future use
	h.oauthService.StoreUserToken(userInfo.Email, token)

	// Call auth service Google OAuth login endpoint
	sessionToken, err := h.callGoogleOAuthLogin(c.Context(), userInfo)
	if err != nil {
		log.Printf("Google OAuth login failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create session",
		})
	}

	type User struct {
		DisplayName string `json:"displayName"`
		AvatarUrl   string `json:"avatar_url"`
	}
	basic_profile := User{DisplayName: userInfo.Name, AvatarUrl: userInfo.Picture}

	// Publish Google login event
	err = h.eventPublisher.PublishGoogleLogin(c.Context(), userInfo.Email, userInfo.Name, userInfo.Picture, userInfo.Locale)
	if err != nil {
		log.Printf("Error publishing event `google.login`: %v", err)
	}

	userDataJSON, _ := json.Marshal(basic_profile)

	// Determine if production environment for secure cookies
	isProduction := len(h.FE_Address) > 8 && h.FE_Address[:8] == "https://"

	// Set session token cookie
	tokenCookie := &fiber.Cookie{
		Name:    "token",
		Value:   sessionToken,
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
		Domain:  ".phrimp.io.vn",
	}

	// Set user data cookie
	userCookie := &fiber.Cookie{
		Name:    "user",
		Value:   string(userDataJSON),
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
		Domain:  ".phrimp.io.vn",
	}

	if isProduction {
		tokenCookie.SameSite = "None"
		tokenCookie.Secure = true
		userCookie.SameSite = "None"
		userCookie.Secure = true
	} else {
		tokenCookie.SameSite = "Lax"
		tokenCookie.Secure = false
		userCookie.SameSite = "Lax"
		userCookie.Secure = false
	}

	c.Cookie(tokenCookie)
	c.Cookie(userCookie)

	return c.Redirect().To(h.FE_Address)
}

// GoogleLoginRequest matches the auth service structure
type GoogleLoginRequest struct {
	Email    string            `json:"email"`
	Name     string            `json:"name"`
	Picture  string            `json:"picture"`
	GoogleID string            `json:"google_id"`
	Locale   string            `json:"locale"`
	Profile  map[string]string `json:"profile"`
}

func (h *AuthHandler) callGoogleOAuthLogin(ctx context.Context, userInfo *models.GoogleUserInfo) (string, error) {
	authServiceURL, err := h.getAuthServiceURL()
	if err != nil {
		return "", fmt.Errorf("failed to get auth service URL: %w", err)
	}

	// Create Google login request
	loginRequest := GoogleLoginRequest{
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Picture:  userInfo.Picture,
		GoogleID: userInfo.ID,
		Locale:   userInfo.Locale,
		Profile: map[string]string{
			"fullname":    userInfo.Name,
			"given_name":  userInfo.GivenName,
			"family_name": userInfo.FamilyName,
			"avatar":      userInfo.Picture,
			"locale":      userInfo.Locale,
			"provider":    "google",
			"google_id":   userInfo.ID,
			"verified":    fmt.Sprintf("%t", userInfo.VerifiedEmail),
		},
	}

	requestBody, err := json.Marshal(loginRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login request: %w", err)
	}

	// Create HTTP request with context timeout
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", authServiceURL+"/public/auth/google/login", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check if request was successful
	if resp.StatusCode == http.StatusOK {
		// The response is the session token as plain text
		return string(body), nil
	}

	// Handle error response
	return "", fmt.Errorf("google OAuth login failed with status %d: %s", resp.StatusCode, string(body))
}

// getAuthServiceURL retrieves the auth service URL from service discovery
func (h *AuthHandler) getAuthServiceURL() (string, error) {
	url := fmt.Sprintf("auth-service:%s", os.Getenv("AUTH_PORT"))
	return url, nil
}

func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
