package routes

import (
	"arunika_backend/handlers"
	"arunika_backend/middlewares"
	"arunika_backend/registry"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func SetupRouter(reg *registry.ServiceRegistry, rdb *redis.Client, db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Outermost middleware: panic recovery → consistent JSON errors
	r.Use(middlewares.ErrorMiddleware())
	// Security headers on every response
	r.Use(middlewares.SecurityHeadersMiddleware())

	// CORS — tighten AllowOrigins in production to your actual domain(s)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Health-check endpoint (unauthenticated — used by load balancers / uptime monitors)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handlers.NewAuthHandler(reg.AuthService)
	auth := r.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/signup", authHandler.SignUp)
		auth.GET("/check-availability", authHandler.CheckAvailability)
		auth.POST("/send-otp", authHandler.SendOtp)
		// No JWT middleware: an expired access token is exactly when a
		// refresh is needed, and the refresh token in the body is the
		// credential. Rate-limited so the endpoint can't be used to probe
		// for valid tokens.
		// The limit is per client IP and generous, since a carrier NAT can
		// put many legitimate users behind one address; a single app
		// refreshes at most once per access-token lifetime (15 minutes).
		auth.POST("/refresh-token", middlewares.RateLimitMiddleware(rdb, "refresh-token", 120, 15*time.Minute), authHandler.RefreshToken)
		auth.POST("/logout", middlewares.JWTAuthMiddleware(rdb), authHandler.Logout)
	}
	r.POST("/forgot-password", middlewares.RateLimitMiddleware(rdb, "forgot-password", 5, 15*time.Minute), authHandler.ForgotPassword)
	r.POST("/reset-password", middlewares.RateLimitMiddleware(rdb, "reset-password", 10, 15*time.Minute), authHandler.ResetPassword)
	// Serves the static reset-password page the emailed link opens — same
	// path as the POST above, different HTTP method, no conflict.
	r.GET("/reset-password", authHandler.ResetPasswordPage)

	userHandler := handlers.NewUserHandler(reg.UserService)
	user := r.Group("/user")
	user.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		user.GET("/:id", userHandler.GetUserByID)
		user.PUT("", userHandler.UpdateUser)
	}

	arHandler := handlers.NewArHandler(reg.ArService)
	r.GET("/ar/cards", middlewares.OptionalAuthMiddleware(rdb), arHandler.GetAll)
	r.GET("/ar/cards/:id", middlewares.OptionalAuthMiddleware(rdb), arHandler.FindById)
	r.GET("/ar/categories", arHandler.GetCategories)

	categoryHandler := handlers.NewCategoryHandler(reg.CategoryService)
	r.GET("/categories", middlewares.JWTAuthMiddleware(rdb), categoryHandler.GetCategories)

	dongengHandler := handlers.NewDongengHandler(reg.DongengService)
	r.GET("/dongeng-categories", dongengHandler.GetCategories)
	r.GET("/fairy-tales", middlewares.OptionalAuthMiddleware(rdb), dongengHandler.GetFairyTales)
	r.GET("/fairy-tales/history", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetHistory)
	r.GET("/fairy-tales/:id", middlewares.OptionalAuthMiddleware(rdb), dongengHandler.GetFairyTaleByID)
	r.POST("/fairy-tales/:id/play", middlewares.OptionalAuthMiddleware(rdb), dongengHandler.RecordPlay)
	r.PUT("/fairy-tales/:id/play", middlewares.JWTAuthMiddleware(rdb), dongengHandler.UpdateProgressHandler)

	// Printable PDF — accessible to guests, but filtered to what the
	// requester is entitled to (see PrintableCardHandler.GetPrintablePDF).
	printableHandler := handlers.NewPrintableCardHandler(db, reg.ProductService, reg.EntitlementService)
	r.GET("/ar/printable-pdf", middlewares.OptionalAuthMiddleware(rdb), printableHandler.GetPrintablePDF)

	// ── Tracing ──────────────────────────────────────────────────────────────
	tracingHandler := handlers.NewTracingHandler(reg.TracingService)
	tracing := r.Group("/tracing")
	tracing.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		tracing.GET("/items", tracingHandler.GetItems)
		tracing.POST("/progress", tracingHandler.SaveProgress)
	}

	// ── Counting ─────────────────────────────────────────────────────────────
	countingHandler := handlers.NewCountingHandler(reg.CountingService)
	counting := r.Group("/counting")
	counting.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		counting.GET("/questions", countingHandler.GetQuestions)
		counting.POST("/progress", middlewares.SubscriptionMiddleware(reg.DB), countingHandler.SaveProgress)
	}

	// ── Badges ───────────────────────────────────────────────────────────────
	badgeHandler := handlers.NewBadgeHandler(reg.BadgeService)
	r.GET("/badges", middlewares.JWTAuthMiddleware(rdb), badgeHandler.GetBadges)

	// ── Payment ──────────────────────────────────────────────────────────────
	paymentHandler := handlers.NewPaymentHandler(reg.PaymentService, reg.NotificationService, reg.PremiumPackService, reg.UserService, reg.ProductService)
	r.POST("/payment/webhook", paymentHandler.Webhook) // no JWT — called by Midtrans
	payment := r.Group("/payment")
	payment.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		payment.POST("/create", paymentHandler.CreateTransaction)
		payment.POST("/create-product", paymentHandler.CreateProductTransaction)
	}

	// ── Orders ───────────────────────────────────────────────────────────────
	orderHandler := handlers.NewOrderHandler(reg.OrderService, reg.PaymentService)
	r.GET("/orders", middlewares.JWTAuthMiddleware(rdb), orderHandler.List)
	r.GET("/orders/:id", middlewares.JWTAuthMiddleware(rdb), orderHandler.GetByID)

	// ── App feature flags ─────────────────────────────────────────────────────
	featureFlagHandler := handlers.NewFeatureFlagHandler(reg.FeatureFlagService)
	r.GET("/app/feature-flags", featureFlagHandler.GetPublic)

	// ── Premium Packs ─────────────────────────────────────────────────────────
	premiumPackHandler := handlers.NewPremiumPackHandler(reg.PremiumPackService)
	// Public — no auth required, but OptionalAuthMiddleware attaches a userID
	// when a valid token is present so already-purchased content packs can be
	// filtered out of the response.
	r.GET("/premium/packs", middlewares.OptionalAuthMiddleware(rdb), premiumPackHandler.GetActivePacks)

	// ── Notifications ─────────────────────────────────────────────────────────
	notifHandler := handlers.NewNotificationHandler(reg.NotificationService)
	notif := r.Group("/notifications")
	notif.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		notif.GET("", notifHandler.GetNotifications)
		notif.PATCH("/:id/read", notifHandler.MarkRead)
		notif.POST("/token", notifHandler.RegisterToken)
	}

	// ── Growth ────────────────────────────────────────────────────────────────
	growthHandler := handlers.NewGrowthHandler(reg.GrowthService)
	growth := r.Group("/growth")
	growth.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		growth.POST("", growthHandler.SaveRecord)
		growth.GET("", growthHandler.GetHistory)
		growth.PUT("/:id", growthHandler.UpdateRecord)
	}

	// ── Admin ─────────────────────────────────────────────────────────────────
	adminAuthHandler := handlers.NewAdminAuthHandler(reg.AdminAuthService)
	adminContentHandler := handlers.NewAdminContentHandler(reg.AdminContentService)
	adminAnalyticsHandler := handlers.NewAdminAnalyticsHandler(reg.AdminAnalyticsService)
	adminUserHandler := handlers.NewAdminUserHandler(reg.AdminUserService)
	adminCampaignHandler := handlers.NewAdminCampaignHandler(reg.AdminCampaignService)
	adminPaymentHandler := handlers.NewAdminPaymentHandler(reg.AdminPaymentService)
	adminProductHandler := handlers.NewAdminProductHandler(reg.ProductService)
	adminOrderHandler := handlers.NewAdminOrderHandler(reg.OrderService, reg.PaymentService)
	bannerHandler := handlers.NewBannerHandler(reg.BannerService)

	// Public banner endpoint for mobile app home screen
	r.GET("/banners", bannerHandler.GetActiveBanners)

	adminAuth := r.Group("/admin/auth")
	{
		adminAuth.POST("/login", adminAuthHandler.Login)
		adminAuth.POST("/refresh", adminAuthHandler.Refresh)
		adminAuth.POST("/logout", middlewares.AdminAuthMiddleware(rdb), adminAuthHandler.Logout)
	}

	admin := r.Group("/admin")
	admin.Use(middlewares.AdminAuthMiddleware(rdb))
	{
		// Analytics
		admin.GET("/analytics/dau", adminAnalyticsHandler.GetDAU)
		admin.GET("/analytics/new-users", adminAnalyticsHandler.GetNewUsers)
		admin.GET("/analytics/popular-features", adminAnalyticsHandler.GetPopularFeatures)
		admin.GET("/analytics/payments", adminAnalyticsHandler.GetPaymentMetrics)
		admin.GET("/analytics/subscription-stats", adminAnalyticsHandler.GetSubscriptionStats)

		// Payments (individual transaction history)
		admin.GET("/payments", adminPaymentHandler.List)
		admin.GET("/payments/:id", adminPaymentHandler.Get)

		// Products & Orders
		admin.GET("/products", adminProductHandler.List)
		admin.POST("/products", adminProductHandler.Create)
		admin.GET("/products/:id", adminProductHandler.Get)
		admin.PUT("/products/:id", adminProductHandler.Update)
		admin.DELETE("/products/:id", adminProductHandler.Delete)
		admin.PATCH("/products/:id/active", adminProductHandler.ToggleActive)
		admin.GET("/orders", adminOrderHandler.List)
		admin.POST("/orders/:id/sync", adminOrderHandler.Sync)

		// Users
		admin.GET("/users", adminUserHandler.ListUsers)
		admin.GET("/users/:id", adminUserHandler.GetUserDetail)
		admin.PATCH("/users/:id/permission", adminUserHandler.UpdatePermission)

		// Campaigns
		admin.GET("/campaigns", adminCampaignHandler.List)
		admin.POST("/campaigns", adminCampaignHandler.Dispatch)

		// App feature flags
		admin.GET("/feature-flags", featureFlagHandler.AdminList)
		admin.PATCH("/feature-flags/:key", featureFlagHandler.AdminToggle)

		// Content — Banners
		admin.GET("/content/banners", bannerHandler.List)
		admin.POST("/content/banners", bannerHandler.Create)
		admin.GET("/content/banners/:id", bannerHandler.Get)
		admin.PUT("/content/banners/:id", bannerHandler.Update)
		admin.DELETE("/content/banners/:id", bannerHandler.Delete)
		admin.PATCH("/content/banners/:id/visibility", bannerHandler.ToggleVisibility)
		admin.PATCH("/content/banners/:id/active", bannerHandler.ToggleActive)

		// Content — Fairy Tales
		admin.GET("/content/fairy-tales", adminContentHandler.ListFairyTales)
		admin.POST("/content/fairy-tales", adminContentHandler.CreateFairyTale)
		admin.GET("/content/fairy-tales/:id", adminContentHandler.GetFairyTale)
		admin.PUT("/content/fairy-tales/:id", adminContentHandler.UpdateFairyTale)
		admin.DELETE("/content/fairy-tales/:id", adminContentHandler.DeleteFairyTale)
		admin.PATCH("/content/fairy-tales/:id/visibility", adminContentHandler.ToggleFairyTaleVisibility)

		// Content — AR Cards
		admin.GET("/content/ar-cards", adminContentHandler.ListArCards)
		admin.POST("/content/ar-cards", adminContentHandler.CreateArCard)
		admin.GET("/content/ar-cards/:id", adminContentHandler.GetArCard)
		admin.PUT("/content/ar-cards/:id", adminContentHandler.UpdateArCard)
		admin.DELETE("/content/ar-cards/:id", adminContentHandler.DeleteArCard)
		admin.PATCH("/content/ar-cards/:id/visibility", adminContentHandler.ToggleArCardVisibility)

		// Content — Tracing Items
		admin.GET("/content/tracing-items", adminContentHandler.ListTracingItems)
		admin.POST("/content/tracing-items", adminContentHandler.CreateTracingItem)
		admin.GET("/content/tracing-items/:id", adminContentHandler.GetTracingItem)
		admin.PUT("/content/tracing-items/:id", adminContentHandler.UpdateTracingItem)
		admin.DELETE("/content/tracing-items/:id", adminContentHandler.DeleteTracingItem)
		admin.PATCH("/content/tracing-items/:id/visibility", adminContentHandler.ToggleTracingItemVisibility)

		// Content — Counting Questions
		admin.GET("/content/counting-questions", adminContentHandler.ListCountingQuestions)
		admin.POST("/content/counting-questions", adminContentHandler.CreateCountingQuestion)
		admin.GET("/content/counting-questions/:id", adminContentHandler.GetCountingQuestion)
		admin.PUT("/content/counting-questions/:id", adminContentHandler.UpdateCountingQuestion)
		admin.DELETE("/content/counting-questions/:id", adminContentHandler.DeleteCountingQuestion)
		admin.PATCH("/content/counting-questions/:id/visibility", adminContentHandler.ToggleCountingQuestionVisibility)

		// Content — Badges
		admin.GET("/content/badges", adminContentHandler.ListBadges)
		admin.POST("/content/badges", adminContentHandler.CreateBadge)
		admin.GET("/content/badges/:id", adminContentHandler.GetBadge)
		admin.PUT("/content/badges/:id", adminContentHandler.UpdateBadge)
		admin.DELETE("/content/badges/:id", adminContentHandler.DeleteBadge)
		admin.PATCH("/content/badges/:id/visibility", adminContentHandler.ToggleBadgeVisibility)

		// Content — Categories
		admin.GET("/content/categories", adminContentHandler.ListCategories)
		admin.POST("/content/categories", adminContentHandler.CreateCategory)
		admin.GET("/content/categories/:id", adminContentHandler.GetCategory)
		admin.PUT("/content/categories/:id", adminContentHandler.UpdateCategory)
		admin.DELETE("/content/categories/:id", adminContentHandler.DeleteCategory)
		admin.PATCH("/content/categories/:id/visibility", adminContentHandler.ToggleCategoryVisibility)

		// Content — Dongeng Pages
		admin.GET("/content/dongen-pages", adminContentHandler.ListDongengPages)
		admin.GET("/content/dongen-pages/:id", adminContentHandler.GetDongengPage)
		admin.POST("/content/dongen-pages", adminContentHandler.CreateDongengPage)
		admin.PUT("/content/dongen-pages/:id", adminContentHandler.UpdateDongengPage)
		admin.DELETE("/content/dongen-pages/:id", adminContentHandler.DeleteDongengPage)

		// Content — AR Card Categories
		admin.GET("/content/ar-card-categories", adminContentHandler.ListArCardCategories)
		admin.POST("/content/ar-card-categories", adminContentHandler.CreateArCardCategory)
		admin.GET("/content/ar-card-categories/:id", adminContentHandler.GetArCardCategory)
		admin.PUT("/content/ar-card-categories/:id", adminContentHandler.UpdateArCardCategory)
		admin.DELETE("/content/ar-card-categories/:id", adminContentHandler.DeleteArCardCategory)
		admin.PATCH("/content/ar-card-categories/:id/visibility", adminContentHandler.ToggleArCardCategoryVisibility)

		// Content — Dongeng Categories
		admin.GET("/content/dongeng-categories", adminContentHandler.ListDongengCategories)
		admin.POST("/content/dongeng-categories", adminContentHandler.CreateDongengCategory)
		admin.GET("/content/dongeng-categories/:id", adminContentHandler.GetDongengCategory)
		admin.PUT("/content/dongeng-categories/:id", adminContentHandler.UpdateDongengCategory)
		admin.DELETE("/content/dongeng-categories/:id", adminContentHandler.DeleteDongengCategory)
		admin.PATCH("/content/dongeng-categories/:id/visibility", adminContentHandler.ToggleDongengCategoryVisibility)

		// Premium Packs
		admin.GET("/premium/packs", premiumPackHandler.AdminListPacks)
		admin.POST("/premium/packs", premiumPackHandler.AdminCreatePack)
		admin.PUT("/premium/packs/:id", premiumPackHandler.AdminUpdatePack)
		admin.DELETE("/premium/packs/:id", premiumPackHandler.AdminDeletePack)
		admin.PATCH("/premium/packs/:id/visibility", premiumPackHandler.AdminToggleVisibility)
		admin.GET("/premium/packs/:id/items", premiumPackHandler.AdminListPackItems)
		admin.POST("/premium/packs/:id/items", premiumPackHandler.AdminAddPackItem)
		admin.DELETE("/premium/packs/:id/items/:product_id", premiumPackHandler.AdminRemovePackItem)
	}

	return r
}
