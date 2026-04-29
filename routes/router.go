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
	r.GET("/ar/cards/:id", middlewares.JWTAuthMiddleware(rdb), arHandler.FindById)

	categoryHandler := handlers.NewCategoryHandler(reg.CategoryService)
	r.GET("/categories", middlewares.JWTAuthMiddleware(rdb), categoryHandler.GetCategories)

	dongengHandler := handlers.NewDongengHandler(reg.DongengService)
	// List endpoint — no pages in response (lightweight for list views)
	r.GET("/fairy-tales", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTales)
	// Detail endpoint — includes ordered pages for book-style reader
	r.GET("/fairy-tales/:id", middlewares.JWTAuthMiddleware(rdb), dongengHandler.GetFairyTaleByID)

	return r
}
