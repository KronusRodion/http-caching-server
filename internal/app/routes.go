package routes

import (
	"http-caching-server/internal/app/handlers"
	"http-caching-server/internal/app/service"
	"http-caching-server/internal/database"
	"github.com/gorilla/mux"
)

func SetupRoutes(jwtSecret, adminToken string) *mux.Router {

	mux := mux.NewRouter() 
	
	//Сервисы
	tokenService := service.NewTokenService(database.DB, jwtSecret)
	userService := service.NewUserService(database.DB)
	storageService:=service.NewFileStorage("./documents")
	fileService := service.NewFileService(database.DB, storageService)

	//Хэндлеры
	authHandler := handlers.NewAuthHandler(tokenService, userService, adminToken)
	fileHandler:= handlers.NewFileHandler(fileService, storageService, userService, database.DB)

	//Роуты
	mux.HandleFunc("/api/register", authHandler.Registration).Methods("POST")
	mux.HandleFunc("/api/auth", authHandler.Authorization).Methods("POST")
	mux.HandleFunc("/api/auth/{token}", authHandler.DeAuthorization).Methods("DELETE")

	mux.HandleFunc("/api/docs", fileHandler.UploadFile).Methods("POST") //Выгрузка файла на сервер
	mux.HandleFunc("/api/docs", fileHandler.GetFiles).Methods("GET", "HEAD") //Получение списка файлов
	mux.HandleFunc("/api/auth/{id}", fileHandler.GetFile).Methods("GET", "HEAD") //Загрузка файла с сервера
	mux.HandleFunc("/api/docs/{id}",fileHandler.DeleteFile).Methods("DELETE") //Удаление файла

	return mux
}
