package service

import (
	"errors"
	"sync"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)



type TokenService struct {
	db        *pgxpool.Pool
	jwtSecret []byte
	blacklist     map[string]time.Time
    blacklistMux  sync.RWMutex
}

func NewTokenService(db *pgxpool.Pool, jwtSecret string) *TokenService {
	return &TokenService{
		db:                db,
		jwtSecret:         []byte(jwtSecret),
		blacklist:         make(map[string]time.Time),
		blacklistMux:      sync.RWMutex{},
	}
}

func (ts *TokenService) InvalidateToken(token string) {
    ts.blacklistMux.Lock()
    defer ts.blacklistMux.Unlock()
    ts.blacklist[token] = time.Now().Add(time.Minute * 15)
}

func (ts *TokenService) IsTokenInvalid(token string) bool {
    ts.blacklistMux.RLock()
    defer ts.blacklistMux.RUnlock()
    
    expiry, exists := ts.blacklist[token]
    if !exists {
        return true
    }
    
    if time.Now().After(expiry) {
        ts.blacklistMux.Lock()
        delete(ts.blacklist, token)
        ts.blacklistMux.Unlock()
        return false
    }
    
    return false
}


func (ts *TokenService) GenerateAccessToken(login, clientIP string) (string, error) {
	claims := jwt.MapClaims{
		"user_login":   login,
		"client_ip": clientIP,
		"exp":       time.Now().Add(time.Minute * 15).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString(ts.jwtSecret)
}

func (ts *TokenService) VerifyAccessToken(tokenString string)  error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad token algo")
		}
		return ts.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return  errors.New("unvalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("unvalid claims format")
	}

	_, ok = claims["user_login"].(string)
	if !ok {
		return  errors.New("unvalid user_login")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return  errors.New("no exp")
	}

	if time.Now().Unix() > int64(exp) {
		return  errors.New("yoken expired")
	}

	if ok = ts.IsTokenInvalid(tokenString); !ok {
		return  errors.New("token is unvalid")
	}

	return nil
}

