package registry

import (
	"arunika_backend/services"
	"arunika_backend/storage"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceRegistry struct {
	DB                     *gorm.DB
	AuthService            *services.AuthService
	AdminAuthService       *services.AdminAuthService
	AdminContentService    *services.AdminContentService
	AdminAnalyticsService  *services.AdminAnalyticsService
	AdminUserService       *services.AdminUserService
	AdminCampaignService   *services.AdminCampaignService
	AdminPaymentService    *services.AdminPaymentService
	BannerService          *services.BannerService
	UserService            *services.UserService
	ArService              *services.ArService
	CategoryService        *services.CategoryService
	DongengService         *services.DongengService
	TracingService         *services.TracingService
	CountingService        *services.CountingService
	BadgeService           *services.BadgeService
	PaymentService         *services.PaymentService
	NotificationService    *services.NotificationService
	GrowthService          *services.GrowthService
	PremiumPackService     *services.PremiumPackService
	ProductService         *services.ProductService
	EntitlementService     *services.EntitlementService
	OrderService           *services.OrderService
	FeatureFlagService     *services.FeatureFlagService
	StrikePriceService     *services.StrikePriceService
	AccountDeletionService *services.AccountDeletionService
	ConsentService         *services.ConsentService
	AuditService           *services.AuditService
	AdminRoleService       *services.AdminRoleService
	AssetService           *services.AssetService
	HurufService           *services.HurufService
	HurufAdminService      *services.HurufAdminService
	AngkaService           *services.AngkaService
	AngkaAdminService      *services.AngkaAdminService
	// MediaLocalDir is set when media are stored on local disk (dev, e2e)
	// and served by the API under /media.
	MediaLocalDir string
}

func NewServiceRegistry(db *gorm.DB, redis *redis.Client) *ServiceRegistry {
	notificationSvc := services.NewNotificationService(db)
	entitlementSvc := services.NewEntitlementService(db)
	strikeSvc := services.NewStrikePriceService(db)
	productSvc := services.NewProductService(db).WithStrikePricing(strikeSvc)
	orderSvc := services.NewOrderService(db, productSvc)
	mediaCfg := storage.ConfigFromEnv()
	store := storage.MustNew(mediaCfg)
	mediaDir := ""
	if mediaCfg.Driver == storage.DriverLocal {
		mediaDir = mediaCfg.LocalDir
	}
	auditSvc := services.NewAuditService(db)
	assetSvc := services.NewAssetService(db, store, auditSvc)
	return &ServiceRegistry{
		DB:                     db,
		AuthService:            services.NewAuthService(db, redis),
		AdminAuthService:       services.NewAdminAuthService(db, redis),
		AdminContentService:    services.NewAdminContentService(db),
		AdminAnalyticsService:  services.NewAdminAnalyticsService(db, redis),
		AdminUserService:       services.NewAdminUserService(db),
		AdminCampaignService:   services.NewAdminCampaignService(db, notificationSvc),
		AdminPaymentService:    services.NewAdminPaymentService(db),
		BannerService:          services.NewBannerService(db),
		UserService:            services.NewUserService(db),
		ArService:              services.NewArService(db, productSvc, entitlementSvc).WithStrikePricing(strikeSvc),
		CategoryService:        services.NewCategoryService(db),
		DongengService:         services.NewDongengService(db, productSvc, entitlementSvc).WithStrikePricing(strikeSvc),
		TracingService:         services.NewTracingService(db),
		CountingService:        services.NewCountingService(db),
		BadgeService:           services.NewBadgeService(db),
		PaymentService:         services.NewPaymentService(db, entitlementSvc),
		NotificationService:    notificationSvc,
		GrowthService:          services.NewGrowthService(db),
		PremiumPackService:     services.NewPremiumPackService(db, orderSvc).WithStrikePricing(strikeSvc),
		ProductService:         productSvc,
		EntitlementService:     entitlementSvc,
		OrderService:           orderSvc,
		FeatureFlagService:     services.NewFeatureFlagService(db),
		StrikePriceService:     strikeSvc,
		AccountDeletionService: services.NewAccountDeletionService(db),
		ConsentService:         services.NewConsentService(db),
		AuditService:           auditSvc,
		AdminRoleService:       services.NewAdminRoleService(db, auditSvc),
		AssetService:           assetSvc,
		HurufService:           services.NewHurufService(db, assetSvc, entitlementSvc),
		HurufAdminService:      services.NewHurufAdminService(db, assetSvc, auditSvc),
		AngkaService:           services.NewAngkaService(db, assetSvc, entitlementSvc),
		AngkaAdminService:      services.NewAngkaAdminService(db, assetSvc, auditSvc),
		MediaLocalDir:          mediaDir,
	}
}
