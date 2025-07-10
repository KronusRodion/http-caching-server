package config

import (
    "github.com/joho/godotenv"
    "log"
    "os"
)

type Config struct {
	DatabaseURL string
	JWT string
    AdminToken string
}

func LoadConfig() (*Config, error) {
	
    err := godotenv.Load()
    if err != nil {
        log.Printf("Warning: unable to load .env file: %v. Falling back to environment variables.", err)
    }

    databaseURL := os.Getenv("DATABASE_URL")
    jwt := os.Getenv("JWT")
    AdminToken := os.Getenv("ADMIN_TOKEN")

    if databaseURL == "" {
        log.Fatal("DATABASE_URL is not set!")
        databaseURL = "postgres://postgres:1234@localhost:5432/postgres?sslmode=disable"
    }

    if jwt == "" {
        log.Fatal("jwt is not set!")
        jwt = "JWTkey"
    }

    if AdminToken == "" {
        log.Fatal("jwt is not set!")
        jwt = "SECURITY_ADMIN_TOKEN_SDKML;JKAISI"
    }


    return &Config{
        DatabaseURL: databaseURL,
        JWT: jwt,
        AdminToken: AdminToken,
	    }, nil
}