package service

import (
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
	"log"
)

type PermissionService struct {
	PermissionRepo *repository.PermissionRepository
}

func NewPermissionService() *PermissionService {
	p_repo := repository.Repositories_instance.PermissionRepository

	if err := p_repo.InitDefaultPermissions(context.Background()); err != nil {
		log.Printf("Warning: Failed to initialize default permissions: %v", err)
	}

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

func (ps *PermissionService) GetAvailablePermission(ctx context.Context, name string) (*models.Permission, error) {
	return ps.PermissionRepo.FindAvailablePermission(ctx, name)
}

func (ps *PermissionService) GetAllPermission(ctx context.Context) ([]*models.Permission, error) {
	return ps.PermissionRepo.FindAll(ctx)
}
