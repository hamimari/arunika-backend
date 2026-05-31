package registry

import (
	"arunika_backend/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceRegistry struct {
	AuthService        *services.AuthService
	UserService        *services.UserService
	ArService          *services.ArService
	CategoryService    *services.CategoryService
	DongengService     *services.DongengService
	AnimalService      *services.AnimalService
	PaymentService     *services.PaymentService
	PremiumPackService *services.PremiumPackService
	BannerService      *services.BannerService
}

func NewServiceRegistry(db *gorm.DB, redis *redis.Client) *ServiceRegistry {
	return &ServiceRegistry{
		AuthService:        services.NewAuthService(db, redis),
		UserService:        services.NewUserService(db),
		ArService:          services.NewArService(db),
		CategoryService:    services.NewCategoryService(db),
		DongengService:     services.NewDongengService(db),
		AnimalService:      services.NewAnimalService(db),
		PaymentService:     services.NewPaymentService(db),
		PremiumPackService: services.NewPremiumPackService(db),
		BannerService:      services.NewBannerService(db),
	}
}
