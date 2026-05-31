package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KistametL/WMS/backend/internal/config"
	"github.com/KistametL/WMS/backend/internal/database"
	"github.com/KistametL/WMS/backend/internal/middleware"
	"github.com/KistametL/WMS/backend/internal/module/auth"
	"github.com/KistametL/WMS/backend/internal/module/dashboard"
	"github.com/KistametL/WMS/backend/internal/module/fulfillment"
	"github.com/KistametL/WMS/backend/internal/module/inventory"
	"github.com/KistametL/WMS/backend/internal/module/order"
	"github.com/KistametL/WMS/backend/internal/module/product"
	"github.com/KistametL/WMS/backend/internal/module/user"
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
	// Disable trusted-proxy auto-detection to silence Gin's startup warning.
	// Set this to the actual proxy CIDR(s) if the app runs behind a reverse proxy.
	r.SetTrustedProxies(nil) //nolint:errcheck,gosec // nil input never returns an error

	// Public routes — ไม่ต้อง auth
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")

	// Auth routes (login, refresh, logout) — public
	authService := auth.NewService(pool, cfg)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(api)

	// Protected routes — ต้องผ่าน RequireAuth ทุก request
	protected := api.Group("")
	protected.Use(
		middleware.RequireAuth(cfg),
		middleware.RequestTimeout(30*time.Second),
	)
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

		// Inventory module — stock levels, movements
		inventoryService := inventory.NewService(pool)
		inventoryHandler := inventory.NewHandler(inventoryService)
		inventoryHandler.RegisterRoutes(protected)

		// Order module — orders, status transitions, cancellations
		orderService := order.NewService(pool)
		orderHandler := order.NewHandler(orderService)
		orderHandler.RegisterRoutes(protected)

		// Fulfillment module — pick → pack → ship workflow + SKU labels
		fulfillmentService := fulfillment.NewService(pool)
		fulfillmentHandler := fulfillment.NewHandler(fulfillmentService)
		fulfillmentHandler.RegisterRoutes(protected)

		// User module — CRUD users, roles, password management
		userService := user.NewService(pool)
		userHandler := user.NewHandler(userService)
		userHandler.RegisterRoutes(protected)

		// Dashboard module — overview KPIs, order breakdown, fulfillment queue, stock alerts
		dashboardService := dashboard.NewService(pool)
		dashboardHandler := dashboard.NewHandler(dashboardService)
		dashboardHandler.RegisterRoutes(protected)
	}

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("Server starting on %s (env: %s)", addr, cfg.AppEnv)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
