package routes

import (
	"arunika_backend/handlers"
	"arunika_backend/middlewares"
	"arunika_backend/registry"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http"
	"time"
)

func SetupRouter(reg *registry.ServiceRegistry, rdb *redis.Client) *gin.Engine {
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
		auth.POST("/send-otp", authHandler.SendOtp)
		auth.POST("/refresh-token", middlewares.JWTAuthMiddleware(rdb), authHandler.RefreshToken)
		auth.POST("/logout", middlewares.JWTAuthMiddleware(rdb), authHandler.Logout)
	}
	r.POST("/forgot-password", authHandler.ForgotPassword)
	r.POST("/reset-password", authHandler.ResetPassword)

	userHandler := handlers.NewUserHandler(reg.UserService)
	user := r.Group("/user")
	user.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		user.GET("/:id", userHandler.GetUserByID)
		user.PUT("", userHandler.UpdateUser)
	}

	arHandler := handlers.NewArHandler(reg.ArService)
	r.GET("/ar/cards/:id", middlewares.JWTAuthMiddleware(rdb), arHandler.FindById)

	categoryHandler := handlers.NewCategoryHandler(reg.CategoryService)
	r.GET("/categories", middlewares.JWTAuthMiddleware(rdb), categoryHandler.GetCategories)

	dongengHandler := handlers.NewDongengHandler(reg.DongengService)
	// List endpoint — no pages in response (lightweight for list views)
	r.GET("/fairy-tales", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTales)
	// Detail endpoint — includes ordered pages for book-style reader
	r.GET("/fairy-tales/:id", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTaleByID)

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
		// GET /counting/questions is open to all authenticated users; lock is enforced client-side.
		counting.GET("/questions", countingHandler.GetQuestions)
		// POST /counting/progress requires premium subscription (medium & hard levels).
		counting.POST("/progress", middlewares.SubscriptionMiddleware(reg.DB), countingHandler.SaveProgress)
	}

	// ── Badges ───────────────────────────────────────────────────────────────
	badgeHandler := handlers.NewBadgeHandler(reg.BadgeService)
	r.GET("/badges", middlewares.JWTAuthMiddleware(rdb), badgeHandler.GetBadges)

	// ── Payment ──────────────────────────────────────────────────────────────
	paymentHandler := handlers.NewPaymentHandler(reg.PaymentService, reg.NotificationService)
	r.POST("/payment/webhook", paymentHandler.Webhook) // no JWT — called by Midtrans
	payment := r.Group("/payment")
	payment.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		payment.POST("/create", paymentHandler.CreateTransaction)
	}

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
	bannerHandler := handlers.NewBannerHandler(reg.BannerService)

	// Public banner endpoint for mobile app home screen
	r.GET("/banners", middlewares.JWTAuthMiddleware(rdb), bannerHandler.GetActiveBanners)

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

		// Users
		admin.GET("/users", adminUserHandler.ListUsers)
		admin.GET("/users/:id", adminUserHandler.GetUserDetail)
		admin.PATCH("/users/:id/permission", adminUserHandler.UpdatePermission)

		// Campaigns
		admin.POST("/campaigns", adminCampaignHandler.Dispatch)

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
	}

	return r
}
