package services

import (
	"context"
	"encoding/json"
	"fmt"
	"google-service/internal/config"
	"google-service/internal/models"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOAuthService struct {
	config       *config.GoogleOAuthConfig
	oauth2Config *oauth2.Config
}

var TokenStorage map[string]*oauth2.Token

func init() {
	TokenStorage = make(map[string]*oauth2.Token)
}

func NewGoogleOAuthService(config *config.GoogleOAuthConfig) *GoogleOAuthService {
	oauth2Config := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Scopes:       config.Scopes,
		Endpoint:     google.Endpoint,
	}
	return &GoogleOAuthService{
		config:       config,
		oauth2Config: &oauth2Config,
	}
}

func (s *GoogleOAuthService) GetAuthURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state)
}

func (s *GoogleOAuthService) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return s.oauth2Config.Exchange(ctx, code)
}

func (s *GoogleOAuthService) GetUserInfo(token *oauth2.Token) (*models.GoogleUserInfo, error) {
	client := s.oauth2Config.Client(context.Background(), token)

	// Make request to Google's userinfo endpoint
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse user info
	var userInfo models.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

func (s *GoogleOAuthService) StoreUserToken(email string, token *oauth2.Token) {
	fmt.Printf("Storing token for user %s\n", email)
	TokenStorage[email] = token
}

func (s *GoogleOAuthService) GetUserToken(email string) (*oauth2.Token, error) {
	token, ok := TokenStorage[email]
	if !ok {
		return nil, fmt.Errorf("token not found for user %s", email)
	}
	return token, nil
}
