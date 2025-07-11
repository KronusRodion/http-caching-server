package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
    DatabaseURL string        `yaml:"database_url"`
    JWT         string        `yaml:"jwt"`
    AdminToken  string        `yaml:"admin_token"`
    Addr        string        `yaml:"redis_address"` 
    Password    string        `yaml:"redis_password"`
    User        string        `yaml:"redis_user"`
    DB          int           `yaml:"redis_db"`
    MaxRetries  int           `yaml:"max_retries"`
    DialTimeout time.Duration `yaml:"dial_timeout"` 
    Timeout     time.Duration `yaml:"timeout"`       
}

func LoadConfig() (*Config, error) {
	
    err := godotenv.Load()
    if err != nil {
        log.Printf("Warning: unable to load .env file: %v. Falling back to environment variables.", err)
    }

    databaseURL := os.Getenv("DATABASE_URL")
    jwt := os.Getenv("JWT")
    AdminToken := os.Getenv("ADMIN_TOKEN")

    cfg := &Config{
        DatabaseURL: os.Getenv("DATABASE_URL"),
        JWT:         os.Getenv("JWT"),
        AdminToken:  os.Getenv("ADMIN_TOKEN"),
        Addr:        os.Getenv("REDIS_ADDRESS"),
        Password:    os.Getenv("REDIS_PASSWORD"),
        User:        os.Getenv("REDIS_USER"),
        DB:          0, 
        MaxRetries:  3, 
        DialTimeout: 5 * time.Second,
        Timeout:     10 * time.Second,
    }

    if databaseURL == "" {
        log.Fatal("DATABASE_URL is not set!")
        cfg.DatabaseURL = "postgres://postgres:1234@localhost:5432/postgres?sslmode=disable"
    }

    if jwt == "" {
        log.Fatal("jwt is not set!")
        cfg.JWT = "JWTkey"
    }

    if AdminToken == "" {
        log.Fatal("jwt is not set!")
        AdminToken = "SECURITY_ADMIN_TOKEN_SDKMLJKAISI"
    }


    return cfg, nil
}