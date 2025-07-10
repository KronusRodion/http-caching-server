package service

import (
	"context"
	"fmt"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileService struct {
	db *pgxpool.Pool
	userService *UserService
	tokenService *TokenService
}


func NewFileService(db *pgxpool.Pool, us *UserService, ts *TokenService) *FileService {
	return &FileService{
		db: db,
		userService: us,
		tokenService: ts,
	}
}


func (file_s *FileService) UploadFile(
    ctx context.Context,
    meta map[string]interface{},
    fileData []byte,
    json_data map[string]interface{},
) (string, error) {

    name, ok := meta["name"].(string)
    if !ok || name == "" {
        return "", fmt.Errorf("missing or invalid 'name'")
    }

    fileFlag, ok := meta["file"].(bool)
    if !ok || !fileFlag {
        return "", fmt.Errorf("'file' flag must be true")
    }

    public, ok := meta["public"].(bool)
    if !ok {
        return "", fmt.Errorf(" invalid 'public' value")
    }

    mime, ok := meta["mime"].(string)
    if !ok || mime == "" {
        return "", fmt.Errorf("missing or invalid 'mime'")
    }

    token, ok := meta["token"].(string)
    if !ok || token == "" {
        return "", fmt.Errorf("missing or invalid 'token'")
    }
	//Проверяем токен
    creatorID, err := file_s.tokenService.VerifyAccessToken(token, ctx)
    if err != nil {
        return "", fmt.Errorf("failed to verify access token: %w", err)
    }

    if fileData == nil{
        return "", fmt.Errorf("file data is empty")
    }

    tx, err := file_s.db.Begin(ctx)
    if err != nil {
        return "", fmt.Errorf("failed to start transaction: %w", err)
    }
    defer tx.Rollback(ctx) //Роллим если не закоммитили транзакцию

    var fileID int
    err = tx.QueryRow(ctx, `
        INSERT INTO files (file_name, size, created_at, json_data, creator, mime_type, is_public)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id
    `, name, len(fileData), time.Now(), json_data, creatorID, mime, public).Scan(&fileID)

    if err != nil {
        return "", fmt.Errorf("failed to insert file: %w", err)
    }

    if !public {
        if grantRaw, ok := meta["grant"].([]interface{}); ok && grantRaw != nil {
            for _, v := range grantRaw {
                login, ok := v.(string)
                if !ok {
                    return "", fmt.Errorf("invalid type in 'grant' array")
                }

                exists, err := file_s.userService.IsUserExist(creatorID, ctx)
                if err != nil {
                    return "", fmt.Errorf("failed to check user '%s': %w", login, err)
                }
                if !exists {
                    return "", fmt.Errorf("user '%s' does not exist", login)
                }

                // Вставляем грант
                _, err = tx.Exec(ctx, `
                    INSERT INTO grants (file_id, user_id)
                    VALUES ($1, $2)
                    ON CONFLICT (file_id, user_id) DO NOTHING
                `, fileID, creatorID)

                if err != nil {
                    return "", fmt.Errorf("failed to insert grant for '%s': %w", login, err)
                }
            }
        }
    }

    if err := tx.Commit(ctx); err != nil {
        return "", fmt.Errorf("failed to commit transaction: %w", err)
    }

    now := time.Now().Format("2006-01-02_15-04-05")
    path := fmt.Sprintf("%d_%s_%s", creatorID, name, now)

    return path, nil
}

