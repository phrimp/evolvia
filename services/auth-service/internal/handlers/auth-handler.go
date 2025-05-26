package handlers

import (
	"auth_service/internal/models"
	"auth_service/internal/service"
	"context"
	"log"
	"time"

	grpcServer "auth_service/internal/grpc"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type ResponseStruct struct {
	message string
	data    map[any]any
}

type AuthHandler struct {
	userService     *service.UserService
	sessionService  *service.SessionService
	userRoleService *service.UserRoleService
	jwtService      *service.JWTService
	gRPCService     *grpcServer.SessionSenderService
}

func NewAuthHandler(userService *service.UserService, jwtService *service.JWTService, sessionService *service.SessionService, userRoleService *service.UserRoleService, grpc *grpcServer.SessionSenderService) *AuthHandler {
	return &AuthHandler{
		userService:     userService,
		jwtService:      jwtService,
		sessionService:  sessionService,
		userRoleService: userRoleService,
		gRPCService:     grpc,
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

	if registerRequest.Username == "" || registerRequest.Email == "" || registerRequest.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email, and password are required",
		})
	}

	if name, ok := registerRequest.Profile["fullname"]; !ok || name == "" {
		first, ok_first := registerRequest.Profile["firstName"]
		last, ok_last := registerRequest.Profile["lastName"]
		if !ok_first && !ok_last {
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error": "First name or Last name are required",
			})
		}
		name = first + " " + last
		registerRequest.Profile["fullname"] = name
	}

	user := &models.UserAuth{
		ID:              bson.NewObjectID(),
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

	err = h.userRoleService.AssignDefaultRoleToUser(c.Context(), user.ID)
	if err != nil {
		log.Printf("Warning: Failed to assign default role to user: %v", err)
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

	login_data, err := h.userService.Login(c.Context(), loginRequest.Username, loginRequest.Password)
	if err != nil {
		log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}
	user_id := login_data["user_id"].(bson.ObjectID)

	permissions, err := h.userRoleService.GetUserPermissions(c.Context(), user_id, "", bson.NilObjectID)
	if err != nil {
		log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Service Error",
		})
	}
	session, err := h.sessionService.GetSession(c.Context(), login_data["username"].(string))
	if err != nil {
		session, err = h.sessionService.NewSession(&models.Session{}, permissions, c.Get("User-Agent"), login_data["username"].(string), login_data["email"].(string))
		if err != nil {
			log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Service Error",
			})
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i := range 5 {
			err = h.gRPCService.SendSession(ctx, session, "middleware")
			if err != nil {
				log.Printf("Error login with username: %s : %s -- Retry: %v", loginRequest.Username, err, i)
			} else {
				log.Printf("Successfully sent session to middleware")
				return
			}
		}
	}()

	// Processing Basic Profile Data
	basic_profile := login_data["basic_profile"].(models.UserProfile)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "None",
		"data": fiber.Map{
			"token":        session.Token,
			"basicProfile": basic_profile,
		},
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
