package repository

import "auth_service/internal/database/mongo"

type Repositories struct {
	PermissionRepository *PermissionRepository
	RedisRepository      *RedisRepo
	RoleRepository       *RoleRepository
	SessionRepository    *SessionRepository
	UserAuthRepository   *UserAuthRepository
	UserRoleRepository   *UserRoleRepository
}

var Repositories_instance = &Repositories{
	PermissionRepository: NewPermissionRepository(mongo.Mongo_Database),
	RedisRepository:      NewRedisRepo(),
	RoleRepository:       NewRoleRepository(mongo.Mongo_Database),
	SessionRepository:    NewSessionRepository(),
	UserAuthRepository:   NewUserAuthRepository(mongo.Mongo_Database),
	UserRoleRepository:   NewUserRoleRepository(mongo.Mongo_Database),
}
