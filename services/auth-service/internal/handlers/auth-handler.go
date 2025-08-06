package handlers

import (
	"auth_service/internal/config"
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"auth_service/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	grpcServer "auth_service/internal/grpc"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	// Counter for total login attempts
	loginAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_login_attempts_total",
			Help: "Total number of login attempts",
		},
		[]string{"status", "method"}, // status: success/failure, method: regular/google
	)

	// Counter for registrations
	registrationAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_registration_attempts_total",
			Help: "Total number of registration attempts",
		},
		[]string{"status", "method"}, // status: success/failure, method: regular/google
	)

	// Histogram for login duration
	loginDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_login_duration_seconds",
			Help:    "Time spent processing login requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	// Gauge for active sessions
	activeSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "auth_active_sessions_current",
			Help: "Current number of active sessions",
		},
	)

	// Counter for logout events
	logoutAttempts = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_logout_attempts_total",
			Help: "Total number of logout attempts",
		},
	)
)

type ResponseStruct struct {
	message string
	data    map[any]any
}

type AuthHandler struct {
	userService        *service.UserService
	sessionService     *service.SessionService
	userRoleService    *service.UserRoleService
	roleService        *service.RoleService
	jwtService         *service.JWTService
	gRPCSessionService *grpcServer.SessionSenderService
	gRPCGoogleService  *grpcServer.GoogleAuthService
	FeAddress          string
}

func NewAuthHandler(userService *service.UserService, jwtService *service.JWTService, sessionService *service.SessionService, userRoleService *service.UserRoleService, grpcSession *grpcServer.SessionSenderService, grpcGoogle *grpcServer.GoogleAuthService, roleService *service.RoleService) *AuthHandler {
	return &AuthHandler{
		userService:        userService,
		jwtService:         jwtService,
		sessionService:     sessionService,
		userRoleService:    userRoleService,
		gRPCSessionService: grpcSession,
		gRPCGoogleService:  grpcGoogle,
		roleService:        roleService,
		FeAddress:          config.ServiceConfig.FEAddress,
	}
}

func (h *AuthHandler) GoogleLoginCallBack(c fiber.Ctx) error {
	var internal_state string
	state_key := "google-auth-state:" + c.Query("state")
	err := repository.Repositories_instance.RedisRepository.GetStructCached(c.Context(), state_key, "", &internal_state)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "No State Found",
		})
	}

	log.Printf("check state: %s with state %s", internal_state, c.Query("state"))
	if internal_state != c.Query("state") {
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
	var user_profile *models.UserProfile
	var avatar_url string

	for i := range 5 {
		user_profile, avatar_url, err = h.gRPCGoogleService.SendGoogleCallBackCode(c.Context(), "google-service", code)
		if err != nil {
			log.Printf("Error google auth: %s -- Retry: %v", err, i)
		} else {
			log.Printf("Successfully sent session to middleware")
			break
		}
	}
	log.Println(user_profile, avatar_url)
	return nil
}

func (h *AuthHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HealthCheck)

	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	authGroup := app.Group("/public/auth")

	authGroup.Post("/register", h.Register)
	authGroup.Post("/login", h.Login)
	authGroup.Post("/login/token", h.LoginWToken)
	authGroup.Post("/internal/login", h.InternalLogin)
	authGroup.Post("/google/login", h.GoogleOAuthLogin)
	// authGroup.Post("/logout", h.Logout)

	app.Post("/protected/auth/logout", h.Logout)
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var registerRequest struct {
		Username string            `json:"username"`
		Email    string            `json:"email"`
		Password string            `json:"password"`
		Profile  map[string]string `json:"profile"`
	}

	if err := c.Bind().Body(&registerRequest); err != nil {
		// Track failed registration attempt due to bad request
		registrationAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if registerRequest.Username == "" || registerRequest.Email == "" || registerRequest.Password == "" {
		// Track failed registration attempt due to missing fields
		registrationAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email, and password are required",
		})
	}

	if name, ok := registerRequest.Profile["fullname"]; !ok || name == "" {
		first, ok_first := registerRequest.Profile["firstName"]
		last, ok_last := registerRequest.Profile["lastName"]
		if !ok_first && !ok_last {
			// Track failed registration attempt due to missing name
			registrationAttempts.WithLabelValues("failure", "regular").Inc()
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
		// Track failed registration attempt due to service error
		registrationAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = h.userRoleService.AssignDefaultRoleToUser(c.Context(), user.ID)
	if err != nil {
		log.Printf("Warning: Failed to assign default role to user: %v", err)
	}

	// Track successful registration
	registrationAttempts.WithLabelValues("success", "regular").Inc()

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User Created Successfully",
		"data": fiber.Map{
			"success": success,
		},
	})
}

func (h *AuthHandler) InternalLogin(c fiber.Ctx) error {
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
		session, err = h.sessionService.NewSession(&models.Session{}, permissions, c.Get("User-Agent"), login_data["username"].(string), login_data["email"].(string), user_id.String())
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
			err = h.gRPCSessionService.SendSession(ctx, session, "middleware")
			if err != nil {
				log.Printf("Error login with username: %s : %s -- Retry: %v", loginRequest.Username, err, i)
			} else {
				log.Printf("Successfully sent session to middleware")
				return
			}
		}
	}()

	return c.Status(fiber.StatusOK).SendString(session.Token)
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
		session, err = h.sessionService.NewSession(&models.Session{}, permissions, c.Get("User-Agent"), login_data["username"].(string), login_data["email"].(string), user_id.String())
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
			err = h.gRPCSessionService.SendSession(ctx, session, "middleware")
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

	isProduction := strings.HasPrefix(h.FeAddress, "https://")

	token := &fiber.Cookie{
		Name:    "token",
		Value:   session.Token,
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
		Domain:  ".phrimp.io.vn",
	}

	userDataJSON, _ := json.Marshal(basic_profile)

	userCookie := &fiber.Cookie{
		Name:    "user",
		Value:   string(userDataJSON),
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
		Domain:  ".phrimp.io.vn",
	}

	if isProduction {
		token.SameSite = "None"
		token.Secure = true
		userCookie.SameSite = "None"
		userCookie.Secure = true

	} else {
		token.SameSite = "Lax"
		token.Secure = false
		userCookie.SameSite = "Lax"
		userCookie.Secure = false
	}

	c.Cookie(token)
	c.Cookie(userCookie)

	//	return c.Status(fiber.StatusOK).JSON(fiber.Map{
	//		"message": "User Login Successfully",
	//		"data": fiber.Map{
	//			"token":        session.Token,
	//			"basicProfile": basic_profile,
	//		},
	//	})
	return c.Redirect().To(h.FeAddress)
}

func (h *AuthHandler) LoginWToken(c fiber.Ctx) error {
	timer := prometheus.NewTimer(loginDuration.WithLabelValues("pending"))
	defer timer.ObserveDuration()

	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind().Body(&loginRequest); err != nil {
		loginAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if loginRequest.Username == "" || loginRequest.Password == "" {
		loginAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username and password are required",
		})
	}

	login_data, err := h.userService.Login(c.Context(), loginRequest.Username, loginRequest.Password)
	if err != nil {
		loginAttempts.WithLabelValues("failure", "regular").Inc()
		log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}
	user_id := login_data["user_id"].(bson.ObjectID)

	permissions, err := h.userRoleService.GetUserPermissions(c.Context(), user_id, "", bson.NilObjectID)
	if err != nil {
		log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
		loginAttempts.WithLabelValues("failure", "regular").Inc()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Service Error",
		})
	}
	session, err := h.sessionService.GetSession(c.Context(), login_data["username"].(string))
	if err != nil {
		session, err = h.sessionService.NewSession(&models.Session{}, permissions, c.Get("User-Agent"), login_data["username"].(string), login_data["email"].(string), user_id.String())
		if err != nil {
			loginAttempts.WithLabelValues("failure", "regular").Inc()
			log.Printf("Error login with username: %s : %s", loginRequest.Username, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Service Error",
			})
		}
		activeSessions.Inc()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i := range 5 {
			err = h.gRPCSessionService.SendSession(ctx, session, "middleware")
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
	userroles, err := h.userRoleService.GetUserRoles(c.Context(), user_id)
	if err != nil {
		log.Printf("error get user role by id: %v. Detail: %v", user_id, err)
	} else {
		for _, userrole := range userroles {
			role, err := h.roleService.GetRoleByID(c.Context(), userrole.RoleID)
			if err != nil {
				log.Printf("retrieving role by id error: %v", err)
			}
			basic_profile.Roles = append(basic_profile.Roles, role.Name)
		}
	}

	loginAttempts.WithLabelValues("success", "regular").Inc()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User Login Successfully",
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
	username := c.Get("X-User-Name")
	log.Printf(c.Get("X-User-Name", "null"), c.Get("X-User-ID", "null"), c.Get("X-User-Email", "null"), c.Get("X-User-Permissions", "null"))
	log.Printf("username: %s", username)
	logoutAttempts.Inc()
	activeSessions.Dec()
	err := h.sessionService.InvalidateSession(c.Context(), username)
	if err != nil {
		log.Printf("invalidate user: %s session failed: %s", username, err)
	}
	null_session := &models.Session{}

	go func() {
		null_session.IPAddress = "invalidate"
		null_session.Token = token
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i := range 5 {
			err := h.gRPCSessionService.SendSession(ctx, null_session, "middleware")
			if err != nil {
				log.Printf("Error logout with username: %s : %s -- Retry: %v", username, err, i)
			} else {
				log.Printf("Successfully sent logout signal to middleware")
				return
			}
		}
	}()

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
	fmt.Printf("token: %s", auth)
	return auth
}

type GoogleLoginRequest struct {
	Email    string            `json:"email"`
	Name     string            `json:"name"`
	Picture  string            `json:"picture"`
	GoogleID string            `json:"google_id"`
	Locale   string            `json:"locale"`
	Profile  map[string]string `json:"profile"`
}

func (h *AuthHandler) GoogleOAuthLogin(c fiber.Ctx) error {
	var googleLoginRequest GoogleLoginRequest

	if err := c.Bind().Body(&googleLoginRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if googleLoginRequest.Email == "" || googleLoginRequest.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and name are required",
		})
	}

	// Try to login with email as both username and password
	login_data, err := h.userService.Login(c.Context(), googleLoginRequest.Email, googleLoginRequest.Email)
	if err != nil {
		// User doesn't exist, create new user
		user := &models.UserAuth{
			ID:              bson.NewObjectID(),
			Username:        googleLoginRequest.Email,
			Email:           googleLoginRequest.Email,
			PasswordHash:    googleLoginRequest.Email, // Set password hash as email
			IsActive:        true,
			IsEmailVerified: true,
			CreatedAt:       int(time.Now().Unix()),
			UpdatedAt:       int(time.Now().Unix()),
		}

		// Create profile data
		profile := googleLoginRequest.Profile
		if profile == nil {
			profile = make(map[string]string)
		}
		profile["fullname"] = googleLoginRequest.Name
		profile["avatar"] = googleLoginRequest.Picture
		profile["locale"] = googleLoginRequest.Locale
		profile["provider"] = "google"
		profile["google_id"] = googleLoginRequest.GoogleID

		success, err := h.userService.Register(c.Context(), user, profile)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if !success {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to create user",
			})
		}

		// Assign default role to new user
		err = h.userRoleService.AssignDefaultRoleToUser(c.Context(), user.ID)
		if err != nil {
			log.Printf("Warning: Failed to assign default role to user: %v", err)
		}

		// Try login again after user creation
		login_data, err = h.userService.Login(c.Context(), googleLoginRequest.Email, googleLoginRequest.Email)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to login after user creation",
			})
		}
	}

	// Rest of session creation logic remains the same...
	user_id := login_data["user_id"].(bson.ObjectID)

	permissions, err := h.userRoleService.GetUserPermissions(c.Context(), user_id, "", bson.NilObjectID)
	if err != nil {
		log.Printf("Error getting permissions for user: %s : %s", googleLoginRequest.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Service Error",
		})
	}

	session, err := h.sessionService.GetSession(c.Context(), login_data["username"].(string))
	if err != nil {
		session, err = h.sessionService.NewSession(
			&models.Session{},
			permissions,
			c.Get("User-Agent"),
			login_data["username"].(string),
			login_data["email"].(string),
			user_id.String(),
		)
		if err != nil {
			log.Printf("Error creating session for user: %s : %s", googleLoginRequest.Email, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Service Error",
			})
		}
	}

	// Send session to middleware via gRPC (async)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i := range 5 {
			err = h.gRPCSessionService.SendSession(ctx, session, "middleware")
			if err != nil {
				log.Printf("Error sending session to middleware for user: %s : %s -- Retry: %v", googleLoginRequest.Email, err, i)
			} else {
				log.Printf("Successfully sent session to middleware for user: %s", googleLoginRequest.Email)
				return
			}
		}
	}()

	return c.Status(fiber.StatusOK).SendString(session.Token)
}
