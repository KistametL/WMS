package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/KistametL/WMS/backend/internal/config"
	"github.com/KistametL/WMS/backend/internal/database"
	"github.com/KistametL/WMS/backend/internal/module/auth"
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

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")

	authService := auth.NewService(pool, cfg)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(api)

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("Server starting on %s (env: %s)", addr, cfg.AppEnv)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
