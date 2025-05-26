package service

import (
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RoleService struct {
	roleRepo       *repository.RoleRepository
	permissionRepo *repository.PermissionRepository
}

func NewRoleService() *RoleService {
	roleRepo := repository.Repositories_instance.RoleRepository
	permissionRepo := repository.Repositories_instance.PermissionRepository

	if err := roleRepo.CollectRoles(context.Background()); err != nil {
		log.Printf("Warning: Failed to load roles into cache: %v", err)
	}

	return &RoleService{
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
	}
}

func (s *RoleService) CreateRole(ctx context.Context, name, description string, permissions []string, isSystem bool) (*models.Role, error) {
	validPermissions := make([]string, 0, len(permissions))
	for _, permName := range permissions {
		perm, err := s.permissionRepo.FindAvailablePermission(ctx, permName)
		if err != nil {
			return nil, fmt.Errorf("invalid permission '%s': %w", permName, err)
		}
		validPermissions = append(validPermissions, perm.Name)
	}

	role := &models.Role{
		Name:        name,
		Description: description,
		Permissions: validPermissions,
		IsSystem:    isSystem,
	}

	return s.roleRepo.Create(ctx, role)
}

func (s *RoleService) UpdateRole(ctx context.Context, id bson.ObjectID, name, description string, permissions []string, isSystem bool) (*models.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if role.IsSystem && !isSystem {
		return nil, errors.New("cannot modify system role status")
	}

	validPermissions := make([]string, 0, len(permissions))
	for _, permName := range permissions {
		perm, err := s.permissionRepo.FindAvailablePermission(ctx, permName)
		if err != nil {
			return nil, fmt.Errorf("invalid permission '%s': %w", permName, err)
		}
		validPermissions = append(validPermissions, perm.Name)
	}

	role.Name = name
	role.Description = description
	role.Permissions = validPermissions
	role.IsSystem = isSystem

	err = s.roleRepo.Update(ctx, role)
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (s *RoleService) DeleteRole(ctx context.Context, id bson.ObjectID) error {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if role.IsSystem {
		return errors.New("cannot delete system role")
	}

	return s.roleRepo.Delete(ctx, id)
}

func (s *RoleService) GetRoleByID(ctx context.Context, id bson.ObjectID) (*models.Role, error) {
	return s.roleRepo.FindByID(ctx, id)
}

func (s *RoleService) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	return s.roleRepo.FindByName(ctx, name)
}

func (s *RoleService) GetAllRoles(ctx context.Context, page, limit int) ([]*models.Role, error) {
	return s.roleRepo.FindAll(ctx, page, limit)
}

func (s *RoleService) AddPermissionToRole(ctx context.Context, roleID bson.ObjectID, permissionName string) error {
	role, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		return err
	}

	_, err = s.permissionRepo.FindAvailablePermission(ctx, permissionName)
	if err != nil {
		return fmt.Errorf("invalid permission '%s': %w", permissionName, err)
	}

	if slices.Contains(role.Permissions, permissionName) {
		return nil
	}

	role.Permissions = append(role.Permissions, permissionName)
	return s.roleRepo.Update(ctx, role)
}

func (s *RoleService) RemovePermissionFromRole(ctx context.Context, roleID bson.ObjectID, permissionName string) error {
	role, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		return err
	}

	found := false
	newPermissions := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		if p != permissionName {
			newPermissions = append(newPermissions, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("role does not have permission '%s'", permissionName)
	}

	role.Permissions = newPermissions
	return s.roleRepo.Update(ctx, role)
}

func (s *RoleService) HasPermission(role *models.Role, permissionName string) bool {
	if role == nil {
		return false
	}

	return slices.Contains(role.Permissions, permissionName)
}

func (s *RoleService) CreateDefaultRoles(ctx context.Context) error {
	_, err := s.roleRepo.FindByName(ctx, "admin")
	if err != nil && strings.Contains(err.Error(), "not found") {
		allPermissions, err := s.permissionRepo.FindAll(ctx)
		if err != nil {
			return fmt.Errorf("failed to get all permissions: %w", err)
		}

		permissionNames := make([]string, len(allPermissions))
		for i, p := range allPermissions {
			permissionNames[i] = p.Name
		}

		_, err = s.CreateRole(ctx, "admin", "Administrator with all permissions", permissionNames, true)
		if err != nil {
			return fmt.Errorf("failed to create admin role: %w", err)
		}
		log.Println("Created default admin role")
	} else if err != nil {
		return fmt.Errorf("error checking for admin role: %w", err)
	} else {
		log.Println("Admin role already exists, skipping creation")
	}

	_, err = s.roleRepo.FindByName(ctx, "user")
	if err != nil && strings.Contains(err.Error(), "not found") {
		basicPermissions := []string{"read", "write"}
		_, err = s.CreateRole(ctx, "user", "Basic user role", basicPermissions, true)
		if err != nil {
			return fmt.Errorf("failed to create user role: %w", err)
		}
		log.Println("Created default user role")
	} else if err != nil {
		return fmt.Errorf("error checking for user role: %w", err)
	} else {
		log.Println("User role already exists, skipping creation")
	}

	return nil
}
