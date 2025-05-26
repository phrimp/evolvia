package service

import (
	"context"
	"fmt"
	"log"
	"profile-service/internal/event"
	"profile-service/internal/models"
	"profile-service/internal/reporsitory"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type ProfileService struct {
	profileRepo *reporsitory.ProfileRepository
	publisher   event.Publisher
}

func NewProfileService(profileRepo *reporsitory.ProfileRepository, publisher event.Publisher) *ProfileService {
	return &ProfileService{
		profileRepo: profileRepo,
		publisher:   publisher,
	}
}

// CreateProfile creates a new user profile
func (s *ProfileService) CreateProfile(ctx context.Context, req *models.CreateProfileRequest) (*models.Profile, error) {
	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if profile already exists for this user
	existingProfile, err := s.profileRepo.FindByUserID(ctx, req.UserID)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("failed to check existing profile: %w", err)
	}
	if existingProfile != nil {
		return nil, fmt.Errorf("profile already exists for user %s", req.UserID)
	}

	// Create new profile
	profile := &models.Profile{
		UserID:       req.UserID,
		PersonalInfo: req.PersonalInfo,
		ContactInfo:  req.ContactInfo,
		PrivacySettings: models.PrivacySettings{
			ProfileVisibility:     models.VisibilityPublic,
			ContactInfoVisibility: models.VisibilityPrivate,
			EducationVisibility:   models.VisibilityPublic,
			ActivityVisibility:    models.VisibilityPrivate,
		},
		Metadata: models.Metadata{
			CreatedAt: int(time.Now().Unix()),
			UpdatedAt: int(time.Now().Unix()),
		},
	}

	// Calculate profile completeness
	profile.ProfileCompleteness = s.calculateCompleteness(profile)

	// Save to database
	createdProfile, err := s.profileRepo.New(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Publish profile created event
	//	profileEvent := &models.ProfileEvent{
	//		EventType: models.EventTypeProfileCreated,
	//		ProfileID: createdProfile.ID.Hex(),
	//		UserID:    createdProfile.UserID,
	//		Timestamp: int(time.Now().Unix()),
	//	}

	//	if err := s.publisher.PublishProfileEvent(profileEvent); err != nil {
	//		log.Printf("Failed to publish profile created event: %v", err)
	//	}

	return createdProfile, nil
}

// GetProfile retrieves a profile by ID
func (s *ProfileService) GetProfile(ctx context.Context, profileID string) (*models.Profile, error) {
	if profileID == "" {
		return nil, fmt.Errorf("profile ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(profileID)
	if err != nil {
		return nil, fmt.Errorf("invalid profile ID format: %w", err)
	}

	profile, err := s.profileRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

func (s *ProfileService) GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	profile, err := s.profileRepo.FindByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("profile not found for user %s", userID)
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

func (s *ProfileService) UpdateProfile(ctx context.Context, profileID string, req *models.UpdateProfileRequest) (*models.Profile, error) {
	if profileID == "" {
		return nil, fmt.Errorf("profile ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(profileID)
	if err != nil {
		return nil, fmt.Errorf("invalid profile ID format: %w", err)
	}

	// Get existing profile
	existingProfile, err := s.profileRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get existing profile: %w", err)
	}

	// Track changes for event
	changedFields := []string{}
	oldValues := make(map[string]any)
	newValues := make(map[string]any)

	// Update fields
	updatedProfile := *existingProfile
	updatedProfile.Metadata.UpdatedAt = int(time.Now().Unix())

	if req.ProfileDTO.PersonalInfo != nil {
		if !s.comparePersonalInfo(&existingProfile.PersonalInfo, req.ProfileDTO.PersonalInfo) {
			changedFields = append(changedFields, "personalInfo")
			oldValues["personalInfo"] = existingProfile.PersonalInfo
			newValues["personalInfo"] = req.ProfileDTO.PersonalInfo
			updatedProfile.PersonalInfo = *req.ProfileDTO.PersonalInfo
		}
	}

	if req.ProfileDTO.ContactInfo != nil {
		if !s.compareContactInfo(&existingProfile.ContactInfo, req.ProfileDTO.ContactInfo) {
			changedFields = append(changedFields, "contactInfo")
			oldValues["contactInfo"] = existingProfile.ContactInfo
			newValues["contactInfo"] = req.ProfileDTO.ContactInfo
			updatedProfile.ContactInfo = *req.ProfileDTO.ContactInfo
		}
	}

	if req.ProfileDTO.EducationalBackground != nil {
		if !s.compareEducationalBackground(existingProfile.EducationalBackground, req.ProfileDTO.EducationalBackground) {
			changedFields = append(changedFields, "educationalBackground")
			oldValues["educationalBackground"] = existingProfile.EducationalBackground
			newValues["educationalBackground"] = req.ProfileDTO.EducationalBackground
			updatedProfile.EducationalBackground = req.ProfileDTO.EducationalBackground
		}
	}

	if req.ProfileDTO.PrivacySettings != nil {
		if !s.comparePrivacySettings(&existingProfile.PrivacySettings, req.ProfileDTO.PrivacySettings) {
			changedFields = append(changedFields, "privacySettings")
			oldValues["privacySettings"] = existingProfile.PrivacySettings
			newValues["privacySettings"] = req.ProfileDTO.PrivacySettings
			updatedProfile.PrivacySettings = *req.ProfileDTO.PrivacySettings
		}
	}

	if len(changedFields) == 0 {
		return existingProfile, nil // No changes
	}

	// Recalculate completeness
	oldCompleteness := existingProfile.ProfileCompleteness
	updatedProfile.ProfileCompleteness = s.calculateCompleteness(&updatedProfile)

	if oldCompleteness != updatedProfile.ProfileCompleteness {
		changedFields = append(changedFields, "profileCompleteness")
		oldValues["profileCompleteness"] = oldCompleteness
		newValues["profileCompleteness"] = updatedProfile.ProfileCompleteness
	}

	// Save changes
	savedProfile, err := s.profileRepo.Update(ctx, objectID, &updatedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Publish profile updated event
	//	profileEvent := &models.ProfileEvent{
	//		EventType:     models.EventTypeProfileUpdated,
	//		ProfileID:     savedProfile.ID.Hex(),
	//		UserID:        savedProfile.UserID,
	//		Timestamp:     int(time.Now().Unix()),
	//		ChangedFields: changedFields,
	//		OldValues:     oldValues,
	//		NewValues:     newValues,
	//	}
	//
	//	if err := s.publisher.PublishProfileEvent(profileEvent); err != nil {
	//		log.Printf("Failed to publish profile updated event: %v", err)
	//	}
	//
	//	// Publish completeness changed event if applicable
	//	if oldCompleteness != updatedProfile.ProfileCompleteness {
	//		completenessEvent := &models.ProfileEvent{
	//			EventType: models.EventTypeCompletenessChanged,
	//			ProfileID: savedProfile.ID.Hex(),
	//			UserID:    savedProfile.UserID,
	//			Timestamp: int(time.Now().Unix()),
	//			OldValues: map[string]any{"completeness": oldCompleteness},
	//			NewValues: map[string]any{"completeness": updatedProfile.ProfileCompleteness},
	//		}
	//
	//		if err := s.publisher.PublishProfileEvent(completenessEvent); err != nil {
	//			log.Printf("Failed to publish completeness changed event: %v", err)
	//		}
	//	}

	return savedProfile, nil
}

// DeleteProfile deletes a profile
func (s *ProfileService) DeleteProfile(ctx context.Context, profileID string) error {
	if profileID == "" {
		return fmt.Errorf("profile ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(profileID)
	if err != nil {
		return fmt.Errorf("invalid profile ID format: %w", err)
	}

	// Get profile before deletion for event
	profile, err := s.profileRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("profile not found")
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Delete profile
	if err := s.profileRepo.Delete(ctx, objectID); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// Publish profile deleted event
	profileEvent := &models.ProfileEvent{
		EventType: models.EventTypeProfileDeleted,
		ProfileID: profile.ID.Hex(),
		UserID:    profile.UserID,
		Timestamp: int(time.Now().Unix()),
	}

	if err := s.publisher.PublishProfileEvent(profileEvent); err != nil {
		log.Printf("Failed to publish profile deleted event: %v", err)
	}

	return nil
}

// ListProfiles retrieves profiles with pagination
func (s *ProfileService) ListProfiles(ctx context.Context, page, limit int) ([]*models.Profile, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	profiles, err := s.profileRepo.FindAll(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}

	return profiles, nil
}

// SearchProfiles searches profiles based on query parameters
func (s *ProfileService) SearchProfiles(ctx context.Context, query *models.ProfileSearchQuery) (*models.ProfileSearchResult, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	profiles, totalCount, err := s.profileRepo.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search profiles: %w", err)
	}

	pageCount := int((totalCount + int64(query.PageSize) - 1) / int64(query.PageSize))

	result := &models.ProfileSearchResult{
		Profiles:    profiles,
		TotalCount:  totalCount,
		PageCount:   pageCount,
		CurrentPage: query.Page,
	}

	return result, nil
}

// GetProfileCompleteness calculates and returns profile completeness
func (s *ProfileService) GetProfileCompleteness(ctx context.Context, profileID string) (*models.ProfileCompletenessResponse, error) {
	profile, err := s.GetProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}

	completeness := s.calculateCompleteness(profile)
	missingFields := s.getMissingFields(profile)
	recommendations := s.getRecommendations(profile)

	return &models.ProfileCompletenessResponse{
		Completeness:       completeness,
		MissingFields:      missingFields,
		RecommendedActions: recommendations,
	}, nil
}

// Helper methods

func (s *ProfileService) validateCreateRequest(req *models.CreateProfileRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if req.PersonalInfo.FirstName == "" {
		return fmt.Errorf("first name is required")
	}
	if req.PersonalInfo.LastName == "" {
		return fmt.Errorf("last name is required")
	}
	if req.ContactInfo.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !s.isValidEmail(req.ContactInfo.Email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func (s *ProfileService) isValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func (s *ProfileService) calculateCompleteness(profile *models.Profile) float64 {
	totalFields := 0
	completedFields := 0

	// Personal Info
	totalFields += 6
	if profile.PersonalInfo.FirstName != "" {
		completedFields++
	}
	if profile.PersonalInfo.LastName != "" {
		completedFields++
	}
	if profile.PersonalInfo.DisplayName != "" {
		completedFields++
	}
	if profile.PersonalInfo.DateOfBirth != 0 {
		completedFields++
	}
	if profile.PersonalInfo.Gender != "" {
		completedFields++
	}
	if profile.PersonalInfo.Biography != "" {
		completedFields++
	}

	// Contact Info
	totalFields += 4
	if profile.ContactInfo.Email != "" {
		completedFields++
	}
	if profile.ContactInfo.Phone != "" {
		completedFields++
	}
	if profile.ContactInfo.AlternativeEmail != "" {
		completedFields++
	}
	if profile.ContactInfo.Address != nil && profile.ContactInfo.Address.Country != "" {
		completedFields++
	}

	// Educational Background
	totalFields += 1
	if len(profile.EducationalBackground) > 0 {
		completedFields++
	}

	if totalFields == 0 {
		return 0.0
	}

	return float64(completedFields) / float64(totalFields) * 100.0
}

func (s *ProfileService) getMissingFields(profile *models.Profile) []string {
	var missing []string

	if profile.PersonalInfo.DisplayName == "" {
		missing = append(missing, "displayName")
	}
	if profile.PersonalInfo.DateOfBirth == 0 {
		missing = append(missing, "dateOfBirth")
	}
	if profile.PersonalInfo.Gender == "" {
		missing = append(missing, "gender")
	}
	if profile.PersonalInfo.Biography == "" {
		missing = append(missing, "biography")
	}
	if profile.ContactInfo.Phone == "" {
		missing = append(missing, "phone")
	}
	if len(profile.EducationalBackground) == 0 {
		missing = append(missing, "educationalBackground")
	}

	return missing
}

func (s *ProfileService) getRecommendations(profile *models.Profile) []string {
	var recommendations []string

	if profile.PersonalInfo.Biography == "" {
		recommendations = append(recommendations, "Add a personal biography to help others know more about you")
	}
	if len(profile.EducationalBackground) == 0 {
		recommendations = append(recommendations, "Add your educational background to showcase your qualifications")
	}
	if profile.ContactInfo.Phone == "" {
		recommendations = append(recommendations, "Add your phone number for better connectivity")
	}

	return recommendations
}

// Comparison methods for tracking changes
func (s *ProfileService) comparePersonalInfo(old, new *models.PersonalInfo) bool {
	return old.FirstName == new.FirstName &&
		old.LastName == new.LastName &&
		old.DisplayName == new.DisplayName &&
		old.DateOfBirth == new.DateOfBirth &&
		old.Gender == new.Gender &&
		old.Biography == new.Biography
}

func (s *ProfileService) compareContactInfo(old, new *models.ContactInfo) bool {
	return old.Email == new.Email &&
		old.Phone == new.Phone &&
		old.AlternativeEmail == new.AlternativeEmail
}

func (s *ProfileService) compareEducationalBackground(old, new []models.EducationalBackground) bool {
	if len(old) != len(new) {
		return false
	}
	for i, o := range old {
		n := new[i]
		if o.Institution != n.Institution ||
			o.Degree != n.Degree ||
			o.Field != n.Field ||
			o.StartDate != n.StartDate ||
			o.EndDate != n.EndDate ||
			o.InProgress != n.InProgress {
			return false
		}
	}
	return true
}

func (s *ProfileService) comparePrivacySettings(old, new *models.PrivacySettings) bool {
	return old.ProfileVisibility == new.ProfileVisibility &&
		old.ContactInfoVisibility == new.ContactInfoVisibility &&
		old.EducationVisibility == new.EducationVisibility &&
		old.ActivityVisibility == new.ActivityVisibility
}
