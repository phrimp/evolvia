package service

import (
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserRoleService struct {
	userRoleRepo *repository.UserRoleRepository
	roleRepo     *repository.RoleRepository
	userRepo     *repository.UserAuthRepository
}

func NewUserRoleService() *UserRoleService {
	return &UserRoleService{
		userRoleRepo: repository.Repositories_instance.UserRoleRepository,
		roleRepo:     repository.Repositories_instance.RoleRepository,
		userRepo:     repository.Repositories_instance.UserAuthRepository,
	}
}

func (s *UserRoleService) AssignRoleToUser(
	ctx context.Context,
	userID, roleID, assignedBy bson.ObjectID,
	scopeType string,
	scopeID bson.ObjectID,
	expiresInDays int,
) (*models.UserRole, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user: %w", err)
	}
	if !user.IsActive {
		return nil, errors.New("cannot assign role to inactive user")
	}

	_, err = s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	userRole := &models.UserRole{
		UserID:     userID,
		RoleID:     roleID,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		AssignedBy: assignedBy,
		AssignedAt: int(time.Now().Unix()),
		IsActive:   true,
	}

	if expiresInDays > 0 {
		expiryTime := time.Now().AddDate(0, 0, expiresInDays)
		userRole.ExpiresAt = int(expiryTime.Unix())
	}

	return s.userRoleRepo.Create(ctx, userRole)
}

func (s *UserRoleService) RemoveRoleFromUser(ctx context.Context, userRoleID bson.ObjectID) error {
	userRole, err := s.userRoleRepo.FindByID(ctx, userRoleID)
	if err != nil {
		return err
	}

	if !userRole.IsActive {
		return nil
	}

	return s.userRoleRepo.Deactivate(ctx, userRoleID)
}

func (s *UserRoleService) GetUserRoles(ctx context.Context, userID bson.ObjectID) ([]*models.UserRole, error) {
	return s.userRoleRepo.FindByUserID(ctx, userID)
}

func (s *UserRoleService) GetUserRolesWithScope(
	ctx context.Context,
	userID bson.ObjectID,
	scopeType string,
	scopeID bson.ObjectID,
) ([]*models.UserRole, error) {
	return s.userRoleRepo.FindByUserIDAndScope(ctx, userID, scopeType, scopeID)
}

func (s *UserRoleService) HasRole(ctx context.Context, userID bson.ObjectID, roleName string) (bool, error) {
	userRoles, err := s.userRoleRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	role, err := s.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		return false, err
	}

	for _, ur := range userRoles {
		if ur.RoleID == role.ID && ur.IsActive {
			return true, nil
		}
	}

	return false, nil
}

func (s *UserRoleService) HasPermission(
	ctx context.Context,
	userID bson.ObjectID,
	permissionName string,
	scopeType string,
	scopeID bson.ObjectID,
) (bool, error) {
	var userRoles []*models.UserRole
	var err error

	if scopeType != "" || !scopeID.IsZero() {
		userRoles, err = s.userRoleRepo.FindByUserIDAndScope(ctx, userID, scopeType, scopeID)
	} else {
		userRoles, err = s.userRoleRepo.FindByUserID(ctx, userID)
	}

	if err != nil {
		return false, err
	}

	if len(userRoles) == 0 {
		return false, nil
	}

	for _, userRole := range userRoles {
		role, err := s.roleRepo.FindByID(ctx, userRole.RoleID)
		if err != nil {
			continue
		}

		if slices.Contains(role.Permissions, permissionName) {
			return true, nil
		}
	}
	return true, nil
}

func (s *UserRoleService) RemoveAllUserRoles(ctx context.Context, userID bson.ObjectID) error {
	return s.userRoleRepo.DeactivateUserRoles(ctx, userID)
}

func (s *UserRoleService) AssignDefaultRoleToUser(ctx context.Context, userID bson.ObjectID) error {
	role, err := s.roleRepo.FindByName(ctx, "user")
	if err != nil {
		return fmt.Errorf("default role 'user' not found: %w", err)
	}

	systemID, _ := bson.ObjectIDFromHex("000000000000000000000000")

	_, err = s.AssignRoleToUser(ctx, userID, role.ID, systemID, "", bson.NilObjectID, 0)
	return err
}

func (s *UserRoleService) GetUserPermissions(
	ctx context.Context,
	userID bson.ObjectID,
	scopeType string,
	scopeID bson.ObjectID,
) ([]string, error) {
	var userRoles []*models.UserRole
	var err error

	if scopeType != "" || !scopeID.IsZero() {
		userRoles, err = s.userRoleRepo.FindByUserIDAndScope(ctx, userID, scopeType, scopeID)
	} else {
		userRoles, err = s.userRoleRepo.FindByUserID(ctx, userID)
	}

	if err != nil {
		return nil, err
	}

	permissionMap := make(map[string]bool)
	for _, userRole := range userRoles {
		role, err := s.roleRepo.FindByID(ctx, userRole.RoleID)
		if err != nil {
			continue
		}

		for _, p := range role.Permissions {
			permissionMap[p] = true
		}
	}

	permissions := make([]string, 0, len(permissionMap))
	for p := range permissionMap {
		permissions = append(permissions, p)
	}

	return permissions, nil
}

func (s *UserRoleService) GetUsersWithRole(ctx context.Context, roleName string, page, limit int) ([]bson.ObjectID, error) {
	role, err := s.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		return nil, err
	}

	return s.userRoleRepo.FindUsersWithRole(ctx, role.ID, page, limit)
}
