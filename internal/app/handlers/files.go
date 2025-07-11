package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"http-caching-server/internal/app/service"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type FileHandler struct {
	fileService    *service.FileService
	storageService *service.StorageService
	tokenService   *service.TokenService
	db             *pgxpool.Pool
	userService		*service.UserService
	redisClient    *redis.Client
}

func NewFileHandler(fileService *service.FileService, storageService *service.StorageService, userService *service.UserService, db *pgxpool.Pool, redisClient *redis.Client) *FileHandler {
	return &FileHandler{
		fileService:    fileService,
		storageService: storageService,
		db:             db,
		userService: userService,
		redisClient:    redisClient,
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
		http.Error(w, "error starting transaction", http.StatusInternalServerError)
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
	
	iter := file_handler.redisClient.Scan(r.Context(), 0, "user:files:*", 0).Iterator()
    for iter.Next(r.Context()) {
        file_handler.redisClient.Del(r.Context(), iter.Val())
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

	cacheKey := fmt.Sprintf("user:files:%d:%s:%s:%s:%d", userID, login, key, value, limit)
	cachedList, err := file_handler.redisClient.Get(r.Context(), cacheKey).Result()

	if err == nil {
		var docs []map[string]interface{}
		if err := json.Unmarshal([]byte(cachedList), &docs); err != nil {
			http.Error(w, "Failed to unmarshal cached list", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			w.Header().Set("X-Doc-Count", strconv.Itoa(len(docs)))
			w.WriteHeader(http.StatusOK)
			return
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"docs": docs,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode cached response", http.StatusInternalServerError)
		}
		return
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

	jsonData, _ := json.Marshal(response)
	json.NewEncoder(w).Encode(response)
	file_handler.redisClient.Set(r.Context(), cacheKey, jsonData, 5*time.Minute)
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

    metaCacheKey := fmt.Sprintf("file:meta:%d", file_id)
	contentCacheKey := fmt.Sprintf("file:content:%d", file_id)
	jsonCacheKey := fmt.Sprintf("file:json:%d", file_id)


	cachedMeta, err := file_handler.redisClient.Get(r.Context(), metaCacheKey).Result()
	if err == nil {
		var meta struct {
			Name     string    `json:"name"`
			MIME     string    `json:"mime"`
			Public   bool      `json:"public"`
			Created  time.Time `json:"created"`
			Grant    []int     `json:"grant"`
			JSONData any       `json:"content,omitempty"`
		}
		//метаданные я оставил на случай расширения
		if err := json.Unmarshal([]byte(cachedMeta), &meta); err != nil {
			http.Error(w, "Failed to unmarshal cached metadata", http.StatusInternalServerError)
			return
		}

		cachedContent, err := file_handler.redisClient.Get(r.Context(), contentCacheKey).Bytes()
		if err != nil {
			if err != redis.Nil {
				http.Error(w, "Failed to get cached content", http.StatusInternalServerError)
				return
			}
		}

		cachedJSON, err := file_handler.redisClient.Get(r.Context(), jsonCacheKey).Result()
		if err != nil && err != redis.Nil {
			http.Error(w, "Failed to get cached JSON", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", meta.MIME)
			w.Header().Set("Content-Length", strconv.Itoa(len(cachedContent)))
			w.WriteHeader(http.StatusOK)
			return
		}

		if meta.JSONData != nil {
			w.Header().Set("Content-Type", "multipart/form-data")
			writer := multipart.NewWriter(w)

			part, err := writer.CreateFormFile("file", meta.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, err = part.Write(cachedContent)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if cachedJSON != "" {
				metadataPart, err := writer.CreateFormField("metadata")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				response := map[string]interface{}{
					"data": cachedJSON,
				}
				json.NewEncoder(metadataPart).Encode(response)
			}

			err = writer.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			w.Header().Set("Content-Type", meta.MIME)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", meta.Name))
			w.Write(cachedContent)
		}

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

	meta := struct {
        Name    string    `json:"name"`
        MIME    string    `json:"mime"`
        Public  bool      `json:"public"`
        Created time.Time `json:"created"`
        Grant   []string  `json:"grant"`
        JSON    any       `json:"content,omitempty"`
    }{
        Name:    fileData.Name,
        MIME:    fileData.MIME,
        Public:  fileData.Public,
        Created: fileData.CreatedAt,
        Grant:   fileData.Grant,
        JSON:    fileData.JSONData,
    }
    metaBytes, err := json.Marshal(meta)
	if err != nil {
		http.Error(w, "error while caching data", http.StatusInternalServerError)
		return
	}

	if len(fileData.JSONData) > 0 {
		w.Header().Set("Content-Type", "multipart/form-data")

		writer := multipart.NewWriter(w)
		part, err := writer.CreateFormFile("file", fileData.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = part.Write(fileData.Content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		metadataPart, err := writer.CreateFormField("metadata")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"data": fileData.JSONData,
		}

		json.NewEncoder(metadataPart).Encode(response)

		err = writer.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		file_handler.redisClient.Set(r.Context(), jsonCacheKey, fileData.JSONData, 15*time.Minute)
	} else {
		//Если json нет, то просто с мимом кидаем
		w.Header().Set("Content-Type", fileData.MIME)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileData.Name))
		w.Write(fileData.Content)
	}
	//Кэшируем результаты
    file_handler.redisClient.Set(r.Context(), metaCacheKey, metaBytes, 15*time.Minute)
    file_handler.redisClient.Set(r.Context(), contentCacheKey, fileData.Content, 15*time.Minute)
}




func (file_handler *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {

	token := r.URL.Query().Get("token")
	user_id, err := file_handler.tokenService.VerifyAccessToken(token, r.Context())
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "file id was not found", http.StatusBadRequest)
		return
	}
	file_id, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	tx, err := file_handler.db.Begin(r.Context())
	defer tx.Rollback(r.Context()) //Роллим в самом конце
	if err != nil {
		http.Error(w, "error starting transaction", http.StatusInternalServerError)
		return
	}



	path, err := file_handler.fileService.DeleteFileFromDB(r.Context(), file_id , user_id)
	if path == "" || err != nil {
		http.Error(w, "file was not found", http.StatusInternalServerError)
		return
	}

	err = file_handler.storageService.DeleteFile(r.Context(), path)
	if err != nil {
		http.Error(w, "error deleting file", http.StatusInternalServerError)
		return
	}

	err = tx.Commit(r.Context())
	if err != nil {
		http.Error(w, "error deleting file", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
        "response": map[string]bool{
            token: true,
        },
    }

	//Удаляем кэш файла
	metaKey := fmt.Sprintf("file:meta:%d", file_id)
    contentKey := fmt.Sprintf("file:content:%d", file_id)
    jsonKey := fmt.Sprintf("file:json:%d", file_id)

    deletedKeys := []string{metaKey, contentKey, jsonKey}
    for _, v := range deletedKeys {
        _ = file_handler.redisClient.Del(r.Context(), v).Err()
    }

	iter := file_handler.redisClient.Scan(r.Context(), 0, "user:files:*", 0).Iterator()
    for iter.Next(r.Context()) {
        file_handler.redisClient.Del(r.Context(), iter.Val())
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
