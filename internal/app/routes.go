package routes

import (
	"http-caching-server/internal/app/handlers"
	"http-caching-server/internal/app/service"
	"http-caching-server/internal/database"
	"net/http"
)

func SetupRoutes(jwtSecret, adminToken string) *http.ServeMux {
	mux := http.NewServeMux()
	tokenService := service.NewTokenService(database.DB, jwtSecret)
	userService := service.NewUserService(database.DB)
	authHandler := handlers.NewAuthHandler(tokenService, userService, adminToken)

	mux.HandleFunc(" /api/register", authHandler.Registration)
	mux.HandleFunc("/api/auth", authHandler.Authorization)
	mux.HandleFunc("/api/auth", authHandler.Authorization)

	return mux
}
