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
        return "", fmt.Errorf("invalid 'public' value")
    }

    mime, ok := meta["mime"].(string)
    if !ok || mime == "" {
        return "", fmt.Errorf("missing or invalid 'mime'")
    }

	token, ok := meta["mime"].(string)
    if !ok || mime == "" {
        return "", fmt.Errorf("missing or invalid 'mime'")
    }

	creatorID, err := file_s.tokenService.VerifyAccessToken(token, ctx)
	if err != nil {
			return "", fmt.Errorf("failed to veriefy access token: %w", err)
		}

    exists, err := file_s.userService.IsUserExist(creatorID, ctx)
    if err != nil {
        return "", fmt.Errorf("failed to check user existence: %w", err)
    }
    if !exists {
        return "", fmt.Errorf("user with id %d does not exist", creatorID)
    }

    if fileData == nil {
        return "", fmt.Errorf("file data is empty")
    }

    var grant []string
    if grantRaw, ok := meta["grant"].([]interface{}); !public && ok && grantRaw != nil {
        for _, v := range grantRaw {
            if s, ok := v.(string); ok {
                grant = append(grant, s)
            } else {
                return "", fmt.Errorf("invalid type in 'grant' array")
            }
        }
    }
	fmt.Println(grant)
    now := time.Now().Format("2006-01-02_15-04-05")
    path := fmt.Sprintf("%d_%s_%s", creatorID, name, now)

	file_s.db.Exec(ctx, "INSERT () INTO files VALUES ()", )

    return path, nil
}

