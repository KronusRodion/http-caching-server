package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"http-caching-server/internal/app/service"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileHandler struct {
	fileService    *service.FileService
	storageService *service.StorageService
	tokenService   *service.TokenService
	db             *pgxpool.Pool
	userService		*service.UserService
}

func NewFileHandler(fileService *service.FileService, storageService *service.StorageService, userService *service.UserService, db *pgxpool.Pool) *FileHandler {
	return &FileHandler{
		fileService:    fileService,
		storageService: storageService,
		db:             db,
		userService: userService,
	}
}

//Структуры для ответа на выгрузку файла
type DataResponse struct {
    JSON map[string]interface{} `json:"json,omitempty"`
    File string                 `json:"file"`
}

type Response struct {
    Data DataResponse `json:"data"`
}

func (file_handler *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tx, err := file_handler.db.Begin(r.Context())
	defer tx.Rollback(r.Context()) //Роллим если не закоммитили транзакцию
	if err != nil {
		http.Error(w, "Invalid 'meta' JSON", http.StatusBadRequest)
		return
	}

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

	token, ok := meta["token"].(string)
	if !ok || token == "" {
		http.Error(w, "Failed to get token from url", http.StatusInternalServerError)
		return
	}

	creatorID, err := file_handler.tokenService.VerifyAccessToken(token, r.Context())
	if err != nil {
		http.Error(w, "Failed to veriefy token", http.StatusInternalServerError)
		return
	}

	exists, err := file_handler.userService.IsUserExist(creatorID, r.Context())
	if err != nil {
		http.Error(w, "Failed to veriefy user", http.StatusInternalServerError)
		return
	}

	path, err := file_handler.fileService.UploadFile(r.Context(), meta, fileData, jsonData, creatorID, exists)
	if err != nil {
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	err = file_handler.storageService.SaveFile(r.Context(), fileData, path)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	response := Response{
            Data: DataResponse{
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

	err = tx.Commit(r.Context())
	if err != nil {
		http.Error(w, "Failed to commit request", http.StatusInternalServerError)
		return
	}
}

func (file_handler *FileHandler) GetFiles(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	if r.Method == http.MethodHead {
		w.Header().Set("X-Doc-Count", strconv.Itoa(len(files)))
		w.WriteHeader(http.StatusOK)
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

func (file_handler *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	file_id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	userID, err := file_handler.tokenService.VerifyAccessToken(token, r.Context())
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	fileData, err := file_handler.fileService.GetFileData(r.Context(), file_id, userID)
	if err != nil {
		if errors.Is(err, errors.New("file not found")) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to load file", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", fileData.MIME)
		w.Header().Set("Content-Length", strconv.Itoa(fileData.Size))
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", fileData.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileData.Name))

	w.Write(fileData.Content)

	if len(fileData.JSONData) > 0 {
		response := map[string]interface{}{
			"response": "ok",
			"data": map[string]interface{}{
				"name":    fileData.Name,
				"mime":    fileData.MIME,
				"public":  fileData.Public,
				"created": fileData.CreatedAt.Format("2006-01-02 15:04:05"),
				"grant":   fileData.Grant,
				"content": fileData.JSONData,
			},
		}
		json.NewEncoder(w).Encode(response)
	}
}
