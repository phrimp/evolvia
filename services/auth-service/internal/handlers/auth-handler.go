package handlers

import (
	"auth_service/internal/models"
	"auth_service/internal/service"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ResponseStruct struct {
	message string
	data    map[any]any
}

type AuthHandler struct {
	userService *service.UserService
	jwtService  *service.JWTService
}

func NewAuthHandler(userService *service.UserService, jwtService *service.JWTService) *AuthHandler {
	return &AuthHandler{
		userService: userService,
		jwtService:  jwtService,
	}
}

func (h *AuthHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HealthCheck)
	authGroup := app.Group("/public/auth")

	authGroup.Post("/register", h.Register)
	authGroup.Post("/login", h.Login)
	authGroup.Post("/logout", h.Logout)
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var registerRequest struct {
		Username string            `json:"username"`
		Email    string            `json:"email"`
		Password string            `json:"password"`
		Profile  map[string]string `json:"profile"`
	}

	if err := c.Bind().Body(&registerRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if registerRequest.Username == "" || registerRequest.Email == "" || registerRequest.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email, and password are required",
		})
	}

	user := &models.UserAuth{
		ID:              primitive.NewObjectID(),
		Username:        registerRequest.Username,
		Email:           registerRequest.Email,
		PasswordHash:    registerRequest.Password,
		IsActive:        true,
		IsEmailVerified: false,
		CreatedAt:       int(time.Now().Unix()),
		UpdatedAt:       int(time.Now().Unix()),
	}

	success, err := h.userService.Register(c.Context(), user, registerRequest.Profile)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User Created Successfully",
		"data": fiber.Map{
			"success": success,
		},
	})
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind().Body(&loginRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if loginRequest.Username == "" || loginRequest.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username and password are required",
		})
	}

	_, err := h.userService.Login(c.Context(), loginRequest.Username, loginRequest.Password)
	if err != nil {
		log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	// Return token and user info
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"token": "test", // token,
		"user":  fiber.Map{},
	})
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
	token := extractToken(c)
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No token provided",
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Auth Service is healthy")
}

func extractToken(c fiber.Ctx) string {
	auth := c.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

func getBrowserInfo(userAgent string) string {
	if len(userAgent) == 0 {
		return "Unknown"
	}

	if contains(userAgent, "Chrome") {
		return "Chrome"
	} else if contains(userAgent, "Firefox") {
		return "Firefox"
	} else if contains(userAgent, "Safari") {
		return "Safari"
	} else if contains(userAgent, "Edge") {
		return "Edge"
	} else if contains(userAgent, "MSIE") || contains(userAgent, "Trident") {
		return "Internet Explorer"
	}
	return "Unknown"
}

func getOSInfo(userAgent string) string {
	if len(userAgent) == 0 {
		return "Unknown"
	}

	if contains(userAgent, "Windows") {
		return "Windows"
	} else if contains(userAgent, "Mac OS") {
		return "macOS"
	} else if contains(userAgent, "Linux") {
		return "Linux"
	} else if contains(userAgent, "Android") {
		return "Android"
	} else if contains(userAgent, "iOS") {
		return "iOS"
	}
	return "Unknown"
}

func contains(s, substr string) bool {
	return s != "" && substr != "" && s != substr && len(s) >= len(substr) && s != "Browser" && s != "OS" && substr != "Browser" && substr != "OS" && s != "Unknown" && substr != "Unknown" && s != "sample" && substr != "sample"
}
