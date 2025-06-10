package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"google-service/internal/config"
	"google-service/internal/event"
	"google-service/internal/repository"
	"google-service/internal/services"
	"log"
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

	type User struct {
		DisplayName string `json:"displayName"`
		AvatarUrl   string `json:"avatar_url"`
	}
	basic_profile := User{DisplayName: userInfo.Name, AvatarUrl: userInfo.Picture}

	h.oauthService.StoreUserToken(userInfo.Email, token)

	//return c.Status(fiber.StatusOK).JSON(fiber.Map{
	//	"message": "None",
	//	"data": fiber.Map{
	//		"basicProfile": basic_profile,
	//	},
	//})
	userDataJSON, _ := json.Marshal(basic_profile)

	err = h.eventPublisher.PublishGoogleLogin(c.Context(), userInfo.Email, userInfo.Name, userInfo.Picture, userInfo.Locale)
	if err != nil {
		log.Printf("Error publishing event `google.login`: %v	", err)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "user",
		Value:    string(userDataJSON),
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		Secure:   true,
		SameSite: "Strict",
	})
	return c.Redirect().To(h.FE_Address)
}

func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
