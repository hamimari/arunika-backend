package routes

import (
	"arunika_backend/handlers"
	"arunika_backend/middlewares"
	"arunika_backend/registry"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"net/http"
	"time"
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
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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
	r.GET("/ar/cards", arHandler.GetAll)
	r.GET("/ar/cards/:id", middlewares.JWTAuthMiddleware(rdb), arHandler.FindById)
	r.GET("/ar/categories", arHandler.GetCategories)

	categoryHandler := handlers.NewCategoryHandler(reg.CategoryService)
	r.GET("/categories", middlewares.JWTAuthMiddleware(rdb), categoryHandler.GetCategories)

	dongengHandler := handlers.NewDongengHandler(reg.DongengService)
	// List endpoint — no pages in response (lightweight for list views)
	//r.GET("/fairy-tales", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTales)
	r.GET("/fairy-tales", dongengHandler.GetFairyTales)
	// Detail endpoint — includes ordered pages for book-style reader
	//r.GET("/fairy-tales/:id", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTaleByID)
	r.GET("/fairy-tales/:id", dongengHandler.GetFairyTaleByID)

	animalHandler := handlers.NewAnimalHandler(reg.AnimalService)
	// Animals — optional ?category=ternak|hutan|laut
	//r.GET("/animals", middlewares.JWTAuthMiddleware(rdb), animalHandler.GetAnimals)
	r.GET("/animals", animalHandler.GetAnimals)

	paymentHandler := handlers.NewPaymentHandler(reg.PaymentService)
	payment := r.Group("/payment")
	{
		// Create Snap token (JWT required)
		payment.POST("/create", middlewares.JWTAuthMiddleware(rdb), paymentHandler.CreatePayment)
		// Midtrans webhook callback (no JWT — called by Midtrans servers)
		payment.POST("/callback", paymentHandler.PaymentCallback)
	}

	// Premium Packages — public (no auth)
	premiumPackHandler := handlers.NewPremiumPackHandler(reg.PremiumPackService)
	r.GET("/premium/packs", premiumPackHandler.GetActivePacks)

	// Banners — public (no auth)
	bannerHandler := handlers.NewBannerHandler(reg.BannerService)
	r.GET("/banners", bannerHandler.GetActiveBanners)

	// Printable PDF — public (no auth)
	printableHandler := handlers.NewPrintableCardHandler(db)
	r.GET("/ar/printable-pdf", printableHandler.GetPrintablePDF)

	// Premium Packages — admin CRUD (JWT required)
	adminPacks := r.Group("/admin/premium/packs")
	adminPacks.Use(middlewares.JWTAuthMiddleware(rdb))
	{
		adminPacks.GET("", premiumPackHandler.AdminListPacks)
		adminPacks.POST("", premiumPackHandler.AdminCreatePack)
		adminPacks.PUT("/:id", premiumPackHandler.AdminUpdatePack)
		adminPacks.DELETE("/:id", premiumPackHandler.AdminDeletePack)
		adminPacks.PATCH("/:id/visibility", premiumPackHandler.AdminToggleVisibility)
	}

	return r
}
