package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FileService struct {
	db             *pgxpool.Pool
	storageService *StorageService
}

func NewFileService(db *pgxpool.Pool, storageService *StorageService) *FileService {
	return &FileService{
		db:             db,
		storageService: storageService,
	}
}

func (file_s *FileService) UploadFileToDB(
	ctx context.Context,
	meta map[string]interface{},
	fileData []byte,
	json_data map[string]interface{},
	creatorID int,
	exists bool,
	path string,
	name string,
) error {

	fileFlag, ok := meta["file"].(bool)
	if !ok || !fileFlag {
		return fmt.Errorf("'file' flag must be true")
	}

	public, ok := meta["public"].(bool)
	if !ok {
		return fmt.Errorf(" invalid 'public' value")
	}

	mime, ok := meta["mime"].(string)
	if !ok || mime == "" {
		return fmt.Errorf("missing or invalid 'mime'")
	}

	if fileData == nil {
		return fmt.Errorf("file data is empty")
	}

	//Начинаем транзакцию
	tx, err := file_s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx) //Роллим если не закоммитили транзакцию

	var fileID int

	err = tx.QueryRow(ctx, `
        INSERT INTO files (file_name, size, created_at, json_data, creator, mime_type, is_public, file_path)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id
    `, name, len(fileData), time.Now(), json_data, creatorID, mime, public, path).Scan(&fileID)

	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	if !public {
		if grantRaw, ok := meta["grant"].([]interface{}); ok && grantRaw != nil {
			for _, v := range grantRaw {
				login, ok := v.(string)
				if !ok {
					return fmt.Errorf("invalid type in 'grant' array")
				}
				if !exists {
					return fmt.Errorf("user '%s' does not exist", login)
				}

				// Вставляем грант
				var userID int
				err := tx.QueryRow(ctx, "SELECT id FROM users WHERE login = $1", login).Scan(&userID)
				if err != nil {
					return fmt.Errorf("user %s not found: %w", login, err)
				}
				_, err = tx.Exec(ctx, `
					INSERT INTO grants (file_id, user_id)
					VALUES ($1, $2)
					ON CONFLICT (file_id, user_id) DO NOTHING
				`, fileID, userID)

				if err != nil {
					return fmt.Errorf("failed to insert grants: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (file_s *FileService) GetFilesData(ctx context.Context, userID int, login string, key string, value string, limit int) ([]map[string]interface{}, error) {
	// Составляем поэтапно запрос к БД
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

		allowedColumns := map[string]bool{
			"file_name":  true,
			"mime_type":  true,
			"created_at": true,
		}
		if !allowedColumns[key] {
			return nil, fmt.Errorf("invalid key: %s", key)
		}
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
			id           int
			name         string
			mime         string
			public       bool
			created      time.Time
			grantsString string // Считываем как строку
		)

		if err := rows.Scan(
			&id,
			&name,
			&mime,
			&public,
			&created,
			&grantsString,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		var grants []int
		if err := json.Unmarshal([]byte(grantsString), &grants); err != nil {
			return nil, fmt.Errorf("failed to parse grants: %w", err)
		}

		files = append(files, map[string]interface{}{
			"id":      strconv.Itoa(id),
			"name":    name,
			"mime":    mime,
			"file":    true, //Ставим в true (Сказали, что всегда file загружается)
			"public":  public,
			"created": created.Format("2006-01-02 15:04:05"),
			"grant":   grants,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return files, nil
}

type FileData struct {
	ID        int
	Name      string
	MIME      string
	CreatorID int
	Public    bool
	Size      int
	JSONData  map[string]interface{}
	Content   []byte
	Grant     []string
	CreatedAt time.Time
	Path      string
}

func (file_s *FileService) GetFileData(ctx context.Context, fileID int, userID int) (*FileData, error) {

	row := file_s.db.QueryRow(ctx, `
        SELECT 
            file_name,
            size,
            file_path,
            mime_type, 
            is_public, 
            json_data, 
            creator
        FROM files
        WHERE id = $1
    `, fileID)

	var (
		file FileData
		json map[string]interface{}
	)

	err := row.Scan(
		&file.Name,
		&file.Size,
		&file.Path,
		&file.MIME,
		&file.Public,
		&json,
		&file.CreatorID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file was not found")
		}
		return nil, fmt.Errorf("failed to fetch file: %w", err)
	}

	if !file.Public {
		exists, err := file_s.isUserHaveAccess(ctx, fileID, userID)
		if err != nil {
			return nil, fmt.Errorf("access check failed: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("access denied")
		}
	}

	var content []byte
	reader, err := file_s.storageService.OpenFile(ctx, file.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer reader.Close()

	content, err = io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return &FileData{
		Name:     file.Name,
		Size:     file.Size,
		MIME:     file.MIME,
		Public:   file.Public,
		JSONData: json,
		Content:  content,
	}, nil
}

func (file_s *FileService) isUserHaveAccess(ctx context.Context, fileID, userID int) (bool, error) {
	// Проверка, что пользователь — владелец или в grant
	var exists bool
	err := file_s.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM grants 
            WHERE file_id = $1 AND user_id = $2
        ) OR $3 = (SELECT creator FROM files WHERE id = $1)
    `, fileID, userID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (file_s *FileService) DeleteFileFromDB(ctx context.Context, fileID, user_id int) (string, error) {

	ok, err := file_s.isUserHaveAccess(ctx, fileID, user_id)
	if !ok {
		return "", fmt.Errorf("user have not access: %w", err)
	}

	var filePath string
	err = file_s.db.QueryRow(ctx, "SELECT file_path FROM files WHERE id = $1", fileID).Scan(&filePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("file with ID %d not found", fileID)
		}
		return "", fmt.Errorf("failed to fetch file_path: %w", err)
	}

	_, err = file_s.db.Exec(ctx, "DELETE FROM files WHERE id = $1", fileID)
	if err != nil {
		return "", fmt.Errorf("failed to delete file: %w", err)
	}

	return filePath, nil
}
