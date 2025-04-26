package service

import (
	"auth_service/internal/database/mongo"
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
)

type PermissionService struct {
	PermissionRepo *repository.PermissionRepository
}

func NewPermissionService() *PermissionService {
	p_repo := repository.NewPermissionRepository(mongo.Mongo_Database)
	p_repo.CollectPermissions(context.Background())
	return &PermissionService{
		PermissionRepo: p_repo,
	}
}

func (ps *PermissionService) NewPermission(ctx context.Context, name, category, description string, isSystem bool) error {
	new_permission := &models.Permission{
		Name:        name,
		Category:    category,
		Description: description,
		IsSystem:    isSystem,
	}
	added_new_permission, err := ps.PermissionRepo.New(ctx, new_permission)
	if err != nil {
		return fmt.Errorf("error creating new Permission: %s", err)
	}
	ps.PermissionRepo.AddtoAvailablePermissions(added_new_permission)
	return nil
}

func (ps *PermissionService) GetAvailablePermission(name string) (*models.Permission, error) {
	return ps.PermissionRepo.FindAvailablePermission(name)
}
