package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	db *pgxpool.Pool
}

func NewUserService(db *pgxpool.Pool) *UserService{
	return &UserService{
		db: db,
	}
}

func (us *UserService) CreateUser(login, password string, ctx context.Context)  error {
    
	loginRegex := regexp.MustCompile(`^[a-zA-Z0-9_]{8,20}$`)
    if !loginRegex.MatchString(login) {
        return errors.New("invalid login")
    }
	//Проверка по логину в бд
    var exists bool
    err := us.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE user_login = $1)", login).Scan(&exists)
    if err != nil {
        return fmt.Errorf("failed to check login uniqueness: %w", err)
    }
    if exists {
        return errors.New("login already taken")
    }

    if !us.IsValidPassword(password) {
        return errors.New("password does not meet requirements")
    }

    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }

    var userID int
    err = us.db.QueryRow(ctx, `
        INSERT INTO users (user_login, user_password, registration_date)
        VALUES ($1, $2, $3)
        RETURNING id
    `, login, string(hashedPassword), time.Now()).Scan(&userID)
    
	if err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }

    return  nil
}


func (us *UserService) IsValidPassword(password string) bool {
    if len(password) < 8 {
        return false
    }

    hasLower := false
    hasUpper := false
    hasDigit := false
    hasSpecial := false

    specialChars := "@$!%*?&"
    specialRegex := regexp.MustCompile("[" + regexp.QuoteMeta(specialChars) + "]")

    for _, char := range password {
        switch {
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsDigit(char):
            hasDigit = true
        case specialRegex.MatchString(string(char)):
            hasSpecial = true
        }
    }

    return hasLower && hasUpper && hasDigit && hasSpecial
}


func (us *UserService) VeriefyUser(login, password string, ctx context.Context)  (int, error) {

    var dbPassword string
    var user_id int
    
    err := us.db.QueryRow(ctx, "SELECT user_password, user_id FROM users WHERE user_login = $1", login).Scan(&dbPassword, &user_id)
    if err != nil {
        return -1, fmt.Errorf("failed to check login uniqueness: %w", err)
    }
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return -1, fmt.Errorf("failed to hashing password: %w", err)
    }

    if dbPassword != string(hashedPassword){
        return -1, errors.New("unvalid login or password")
    }

    return user_id, nil
}

func (us *UserService) IsUserExist(user_id int, ctx context.Context)  (bool, error) {

    var exist bool
    err := us.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1", user_id).Scan(&exist)
    if !exist {
        return false, errors.New("no user with same login")
    }

    return true, err
}