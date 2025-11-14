package registry

import (
	"arunika_backend/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceRegistry struct {
	AuthService *services.AuthService
	UserService *services.UserService
}

func NewServiceRegistry(db *gorm.DB, redis *redis.Client) *ServiceRegistry {
	return &ServiceRegistry{
		AuthService: services.NewAuthService(db, redis),
		UserService: services.NewUserService(db),
	}
}
