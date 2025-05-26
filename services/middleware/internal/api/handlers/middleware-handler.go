package handlers

import (
	"log"
	"middleware/internal/services"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type MiddlewareHandler struct {
	sessionService *services.SessionService
}

func NewMiddlewareHandler(sessionService *services.SessionService) *MiddlewareHandler {
	return &MiddlewareHandler{
		sessionService: sessionService,
	}
}

func (h *MiddlewareHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/auth/validate", h.ValidateToken)
}

func (h *MiddlewareHandler) ValidateToken(c fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authorization token",
		})
	}

	tokenString := authHeader
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = authHeader[7:]
	}

	// Validate the token and extract claims
	claims, err := h.sessionService.ValidateToken(tokenString)
	if err != nil {
		log.Printf("Token validation failed: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token: " + err.Error(),
		})
	}

	//	session, err := h.sessionService.GetSession(c.Context(), tokenString)
	//	if err != nil || !session.IsValid {
	//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
	//			"error": "Session not found or invalid",
	//		})
	//	}

	// Set headers for downstream services
	c.Set("X-User-ID", claims.Id)
	c.Set("X-User-Email", claims.Email)
	c.Set("X-User-Name", claims.Username)

	// Set permissions as a comma-separated list
	if len(claims.Permissions) > 0 {
		c.Set("X-User-Permissions", strings.Join(claims.Permissions, ","))
	}

	// Return success status (not c.Next())
	return c.Status(fiber.StatusOK).Send(nil)
}
