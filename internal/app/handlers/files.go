package handlers

import (
	"encoding/json"
	"http-caching-server/internal/app/service"
	"io"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FileHandler struct {
	fileService *service.FileService
	storageService *service.FileStorage
	tokenService *service.TokenService
	db *pgxpool.Pool
}



func NewFileHandler(fileService *service.FileService, storageService *service.FileStorage, 	db *pgxpool.Pool) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		storageService: storageService,
		db: db,
	}
}


func (file_handler *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {

	tx, err := file_handler.db.Begin(r.Context())
    if err != nil {
		http.Error(w, "Invalid 'meta' JSON", http.StatusBadRequest)
		return
    }
    defer tx.Rollback(r.Context()) //Роллим если не закоммитили транзакцию

	//Парсим форму и извлекаем метаданные и json
	metaJSON := r.FormValue("meta")
	if metaJSON == "" {
		http.Error(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		http.Error(w, "Invalid 'meta' JSON", http.StatusBadRequest)
		return
	}

	jsonRaw := r.FormValue("json")
	var jsonData map[string]interface{}

	if jsonRaw != "" {
		if err := json.Unmarshal([]byte(jsonRaw), &jsonData); err != nil {
			http.Error(w, "Invalid 'json' JSON", http.StatusBadRequest)
			return
		}
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	path, err:= file_handler.fileService.UploadFile(r.Context(), meta, fileData, jsonData)
	if err != nil {
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	err = file_handler.storageService.SaveFile(r.Context(), fileData, path)
	if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}


	response := struct {
            Data struct {
                JSON map[string]interface{} `json:"json,omitempty"`
                File string                 `json:"file"`
            } `json:"data"`
        }{
            Data: struct {
                JSON map[string]interface{} `json:"json,omitempty"`
                File string                 `json:"file"`
            }{
                JSON: jsonData,
                File: meta["name"].(string),
            },
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        if err := json.NewEncoder(w).Encode(response); err != nil {
            http.Error(w, "Failed to encode response", http.StatusInternalServerError)
            return
        }

	tx.Commit(r.Context())
}

func (file_handler *FileHandler) GetFiles(w http.ResponseWriter, r *http.Request) {

    token := r.URL.Query().Get("token")
    login := r.URL.Query().Get("login")
    key := r.URL.Query().Get("key")
    value := r.URL.Query().Get("value")
    limitStr := r.URL.Query().Get("limit")

    // Валидируем токен
    userID, err := file_handler.tokenService.VerifyAccessToken(token, r.Context())
    if err != nil {
        http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
        return
    }

    limit := 0
    if limitStr != "" {
        limit64, err := strconv.ParseInt(limitStr, 10, 64)
        if err == nil && limit64 > 0 {
            limit = int(limit64)
        }
    }

    files, err := file_handler.fileService.GetFilesData(r.Context(), userID, login, key, value, limit)
    if err != nil {
        http.Error(w, "Failed to load files", http.StatusInternalServerError)
        return
    }

    response := map[string]interface{}{
        "data": map[string]interface{}{
            "docs": files,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(response)
}