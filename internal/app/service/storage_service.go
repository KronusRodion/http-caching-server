package service

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
)

type StorageService struct {
	basePath string
}

func NewFileStorage(basePath string) *StorageService {
	return &StorageService{basePath}
}

func (s *StorageService) SaveFileToStorage(ctx context.Context, fileData []byte, path string) error {
	fullPath := filepath.Join(s.basePath, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		log.Printf("Error creating directory: %v", err)
		return err
	}
	if _, err := os.Stat(fullPath); err == nil {
		log.Printf("file already exist: %v", err)
		return err
	}
	err = os.WriteFile(fullPath, fileData, 0644)
	if err != nil {
		log.Printf("Error writing file: %v", err)
	}
	return err
}

func (s *StorageService) OpenFile(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)
	file, err := os.Open(fullPath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
	}
	return file, err
}

func (s *StorageService) DeleteFile(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.basePath, path)

	err := os.Remove(fullPath)
	if err != nil {
		log.Println("Error of deleting file with fullPath: ", fullPath)
		return err
	}

	log.Printf("The file with fullPath: %s was successfull deleted", fullPath)
	return nil
}
