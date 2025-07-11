package main

import (
	"context"
	routes "http-caching-server/internal/app"
	"http-caching-server/internal/config"
	"http-caching-server/internal/database"
	"log"
	"net/http"

	"github.com/rs/cors"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load config:", err)
	}

	redis, err := database.NewClient(context.Background(), *cfg)
	if err != nil {
		log.Fatal("Unable to load redis:", err)
	}

	err = database.Init(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Unable to load database:", err)
	}
	defer database.CloseDB()


	mux := routes.SetupRoutes(cfg.JWT, cfg.AdminToken, redis)
	handler := cors.AllowAll().Handler(mux)

	log.Println("Server starting on :80...")
	err = http.ListenAndServe(":80", handler)
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
