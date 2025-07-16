package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type TokenService struct {
    jwtSecret []byte
    redis     *redis.Client
    tokenTTL  time.Duration // Срок жизни токена (например, 7 дней)
}

func NewTokenService(jwtSecret string, redis *redis.Client) *TokenService {
    return &TokenService{
        jwtSecret: []byte(jwtSecret),
        redis:     redis,
        tokenTTL:  7 * 24 * time.Hour, // Токен действителен неделю
    }
}


func (ts *TokenService) GenerateAccessToken(login, clientIP string, userID int) (string, error) {
    expTime := time.Now().Add(ts.tokenTTL).Unix()

    claims := jwt.MapClaims{
        "user_login": login,
        "client_ip":  clientIP,
        "user_id":    userID,
        "exp":        expTime,
        "iat":        time.Now().Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
    signedToken, err := token.SignedString(ts.jwtSecret)
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %w", err)
    }

    return signedToken, nil
}


func (ts *TokenService) VerifyAccessToken(tokenString string, ctx context.Context) (int, error) {
    if ts.isTokenRevoked(ctx, tokenString) {
        return -1, errors.New("token has been revoked")
    }

    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("unexpected signing method")
        }
        return ts.jwtSecret, nil
    })

    if err != nil {
        return -1, fmt.Errorf("invalid token: %w", err)
    }

    if !token.Valid {
        return -1, errors.New("token is not valid")
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return -1, errors.New("invalid token claims")
    }

    // exp, ok := claims["exp"].(float64)
    // if !ok || time.Now().After(time.Unix(int64(exp), 0)) {
    //     return -1, errors.New("token has expired")
    // }

    userIDStr, ok := claims["user_id"].(string)
    if !ok {
        return -1, errors.New("user_id not found in token")
    }
    userID, err := strconv.Atoi(userIDStr)
    if err != nil {
        return -1, errors.New("invalid user_id")
    }

    return int(userID), nil
}

func (ts *TokenService) isTokenRevoked(ctx context.Context, token string) bool {
    tokenHash := sha256.Sum256([]byte(token))
    key := "revoked:" + hex.EncodeToString(tokenHash[:])
    val, err := ts.redis.Get(ctx, key).Result()
    return err == nil && val == "revoked"
}

func (ts *TokenService) RevokeToken(ctx context.Context, tokenString string) error {
    token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
    if err != nil {
        return fmt.Errorf("failed to parse token: %w", err)
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return errors.New("invalid token claims")
    }

    exp, ok := claims["exp"].(float64)
    if !ok {
        return errors.New("token has no expiration time")
    }

    expTime := time.Unix(int64(exp), 0)
    now := time.Now()
    if now.After(expTime) {
        return errors.New("token is already expired")
    }

    remainingTTL := expTime.Sub(now)
    tokenHash := sha256.Sum256([]byte(tokenString))
    key := "revoked:" + hex.EncodeToString(tokenHash[:])

    err = ts.redis.Set(ctx, key, "revoked", remainingTTL).Err()
    if err != nil {
        return fmt.Errorf("failed to revoke token in Redis: %w", err)
    }

    return nil
}

