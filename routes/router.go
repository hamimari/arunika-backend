package routes

import (
	"arunika_backend/handlers"
	"arunika_backend/middlewares"
	"arunika_backend/registry"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func SetupRouter(reg *registry.ServiceRegistry, redis *redis.Client) *gin.Engine {
	r := gin.Default()

	authHandler := handlers.NewAuthHandler(reg.AuthService)
	auth := r.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/signup", authHandler.SignUp)
		auth.POST("/send-otp", authHandler.SendOtp)
		auth.POST("/refresh-token", middlewares.JWTAuthMiddleware(redis), authHandler.RefreshToken)
		auth.POST("/logout", middlewares.JWTAuthMiddleware(redis), authHandler.Logout)
	}
	r.POST("/forgot-password", authHandler.ForgotPassword)
	r.POST("/reset-password", authHandler.ResetPassword)

	userHandler := handlers.NewUserHandler(reg.UserService)
	user := r.Group("/user")
	user.Use(middlewares.JWTAuthMiddleware(redis))
	{
		user.GET("/:id", userHandler.GetUserByID)
		user.PUT("", userHandler.UpdateUser)
	}
	arHandler := handlers.NewArHandler(reg.ArService)
	r.GET("/ar/cards/:id", middlewares.JWTAuthMiddleware(redis), arHandler.FindById)

	categoryHandler := handlers.NewCategoryHandler(reg.CategoryService)
	r.GET("/categories", middlewares.JWTAuthMiddleware(redis), categoryHandler.GetCategories)

	dongengHandler := handlers.NewDongengHandler(reg.DongengService)
	r.GET("/fairy-tales", middlewares.JWTAuthMiddleware(redis), dongengHandler.GetFairyTales)

	return r
}
