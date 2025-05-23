package handlers

import (
	"google-service/internal/config"
	"google-service/internal/services"

	"github.com/gofiber/fiber/v3"
)

type GoogleHandler struct {
	oauthService *services.GoogleOAuthService
}

func NewGoogleHandler(config *config.GoogleOAuthConfig) *GoogleHandler {
	return &GoogleHandler{
		oauthService: services.NewGoogleOAuthService(config),
	}
}

func (h *GoogleHandler) HandleGoogleLogin(c fiber.Ctx) error {
	state := c.Query("state")
	if state == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "State parameter required",
		})
	}

	url := h.oauthService.GetAuthURL(state)
	return c.Redirect().To(url)
}
