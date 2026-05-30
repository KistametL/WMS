package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/KistametL/WMS/backend/internal/config"
	"github.com/KistametL/WMS/backend/internal/database"
	"github.com/KistametL/WMS/backend/internal/middleware"
	"github.com/KistametL/WMS/backend/internal/module/auth"
	"github.com/KistametL/WMS/backend/internal/module/product"
	"github.com/KistametL/WMS/backend/pkg/response"
)

func main() {
	cfg := config.Load()

	pool, err := database.NewPool(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to database")

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Public routes — ไม่ต้อง auth
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")

	// Auth routes (login, refresh, logout) — public
	authService := auth.NewService(pool, cfg)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(api)

	// Protected routes — ต้องผ่าน RequireAuth ทุก request
	protected := api.Group("")
	protected.Use(middleware.RequireAuth(cfg))
	{
		// GET /api/v1/me — ดูข้อมูล user ที่ login อยู่
		// ใช้เป็น smoke test ว่า JWT middleware ทำงานถูกต้อง
		protected.GET("/me", func(c *gin.Context) {
			response.OK(c, gin.H{
				"user_id":     middleware.GetUserID(c),
				"email":       middleware.GetEmail(c),
				"roles":       middleware.GetRoles(c),
				"permissions": middleware.GetPermissions(c),
			})
		})

		// Product module — categories, products, SKUs
		productService := product.NewService(pool)
		productHandler := product.NewHandler(productService)
		productHandler.RegisterRoutes(protected)
	}

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("Server starting on %s (env: %s)", addr, cfg.AppEnv)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
