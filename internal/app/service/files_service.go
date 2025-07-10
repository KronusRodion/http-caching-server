package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

    //Начинаем транзакцию
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


func (file_s *FileService) GetFilePath(id int) {

}

func (file_s *FileService) GetFilesData(ctx context.Context, userID int, login string, key string, value string, limit int) ([]map[string]interface{}, error) {
    query := `
        SELECT 
            f.id, 
            f.file_name AS name, 
            f.mime_type AS mime, 
            f.is_public AS public, 
            f.created_at AS created,
            COALESCE(jsonb_agg(g.user_id) FILTER (WHERE g.user_id IS NOT NULL), '[]') AS grant_list
        FROM files f
        LEFT JOIN grants g ON f.id = g.file_id
    `

    var args []interface{}
    var conditions []string

    if login == "" {
        conditions = append(conditions, fmt.Sprintf("f.creator = $%d", len(args)+1))
        args = append(args, userID)
    } else {
        conditions = append(conditions, fmt.Sprintf("(f.creator = $%d OR g.user_id = $%d)", len(args)+1, len(args)+1))
        args = append(args, userID)
    }

    if key != "" && value != "" {
        conditions = append(conditions, fmt.Sprintf("%s = $%d", key, len(args)+1))
        args = append(args, value)
    }

    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }

    query += `
        GROUP BY f.id, f.file_name, f.mime_type, f.is_public, f.created_at
        ORDER BY f.file_name, f.created_at DESC
    `

    if limit > 0 {
        query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
        args = append(args, limit)
    }

    rows, err := file_s.db.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch files: %w", err)
    }
    defer rows.Close()

    var files []map[string]interface{}

    for rows.Next() {
        var (
            id      int
            name    string
            mime    string
            public  bool
            created time.Time
            grants  []int
        )

        if err := rows.Scan(&id, &name, &mime, &public, &created, &grants); err != nil {
            return nil, fmt.Errorf("scan error: %w", err)
        }

        files = append(files, map[string]interface{}{
            "id":     strconv.Itoa(id),
            "name":   name,
            "mime":   mime,
            "file":   true, // предположим, что это всегда true, как в ТЗ, так как сказали, что file всегда есть
            "public": public,
            "created": created.Format("2006-01-02 15:04:05"),
            "grant":  grants,
        })
    }

    return files, nil
}
