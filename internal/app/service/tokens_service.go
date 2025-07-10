package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)



type TokenService struct {
	db        *pgxpool.Pool
	jwtSecret []byte
}

func NewTokenService(db *pgxpool.Pool, jwtSecret string) *TokenService {
	return &TokenService{
		db:                db,
		jwtSecret:         []byte(jwtSecret),
	}
}



func (ts *TokenService) isTokenInvalid(ctx context.Context, token string) (int, error) {
    var user_id int

    err := ts.db.QueryRow(ctx, `
        SELECT user_id FROM tokens 
            WHERE token = $1 AND expiry > NOW()
        )
    `, token).Scan(&user_id)

    if err != nil {
        return -1, err
    }

    return user_id, nil
}


func (ts *TokenService) GenerateAccessToken(login, clientIP string, user_id int, ctx context.Context) (string, error) {
	
	expTime:=time.Now().Add(time.Hour * 24 * 7).Unix() //токен работает неделю
    claims := jwt.MapClaims{
        "user_login": login,
        "client_ip":  clientIP,
        "exp":        expTime,
        "iat":        time.Now().Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

    signedToken, err := token.SignedString(ts.jwtSecret)
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %w", err)
    }

    _, err = ts.db.Exec(ctx, `
        INSERT INTO tokens (token, user_id, expiry)
        VALUES ($1, $2, $3)
        ON CONFLICT (token) DO NOTHING
    `, signedToken, user_id, expTime)

    if err != nil {
        return "", fmt.Errorf("failed to save token: %w", err)
    }

    return signedToken, nil
}

func (ts *TokenService) VerifyAccessToken(tokenString string, ctx context.Context)  (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad token algo")
		}
		return ts.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return  -1, errors.New("unvalid token")
	}

	user_id, err := ts.isTokenInvalid(ctx, tokenString)
	if err != nil {
		return  -1, errors.New("token is unvalid")
	}

	return user_id, nil
}

func (ts *TokenService) RevokeToken(ctx context.Context, token string) error {

    commandTag, err := ts.db.Exec(ctx, `
        DELETE FROM tokens
        WHERE token = $1
    `, token)

    if err != nil {
        return fmt.Errorf("failed to revoke token: %w", err)
    }

    if commandTag.RowsAffected() == 0 {
        return fmt.Errorf("token not found or already revoked")
    }

    return nil
}

