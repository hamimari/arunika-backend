package registry

import (
	"arunika_backend/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceRegistry struct {
	DB                    *gorm.DB
	AuthService           *services.AuthService
	AdminAuthService      *services.AdminAuthService
	AdminContentService   *services.AdminContentService
	AdminAnalyticsService *services.AdminAnalyticsService
	AdminUserService      *services.AdminUserService
	AdminCampaignService  *services.AdminCampaignService
	AdminPaymentService   *services.AdminPaymentService
	BannerService         *services.BannerService
	UserService           *services.UserService
	ArService             *services.ArService
	CategoryService       *services.CategoryService
	DongengService        *services.DongengService
	TracingService        *services.TracingService
	CountingService       *services.CountingService
	BadgeService          *services.BadgeService
	PaymentService        *services.PaymentService
	NotificationService   *services.NotificationService
	GrowthService         *services.GrowthService
}

func NewServiceRegistry(db *gorm.DB, redis *redis.Client) *ServiceRegistry {
	notificationSvc := services.NewNotificationService(db)
	return &ServiceRegistry{
		DB:                    db,
		AuthService:           services.NewAuthService(db, redis),
		AdminAuthService:      services.NewAdminAuthService(db, redis),
		AdminContentService:   services.NewAdminContentService(db),
		AdminAnalyticsService: services.NewAdminAnalyticsService(db, redis),
		AdminUserService:      services.NewAdminUserService(db),
		AdminCampaignService:  services.NewAdminCampaignService(db, notificationSvc),
		AdminPaymentService:   services.NewAdminPaymentService(db),
		BannerService:         services.NewBannerService(db),
		UserService:           services.NewUserService(db),
		ArService:             services.NewArService(db),
		CategoryService:       services.NewCategoryService(db),
		DongengService:        services.NewDongengService(db),
		TracingService:        services.NewTracingService(db),
		CountingService:       services.NewCountingService(db),
		BadgeService:          services.NewBadgeService(db),
		PaymentService:        services.NewPaymentService(db),
		NotificationService:   notificationSvc,
		GrowthService:         services.NewGrowthService(db),
	}
}
