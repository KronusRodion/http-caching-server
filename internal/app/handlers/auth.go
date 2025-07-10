package handlers

import (
	"encoding/json"
	"http-caching-server/internal/app/service"
	"log"
	"net/http"
	"regexp"
	"github.com/gorilla/mux"
)

type AuthHandler struct {
	tokenService *service.TokenService
	userService *service.UserService
	adminToken string
}

type RegistrationRequest struct {
    Token   string `json:"token"`
    Login   string `json:"login"`
    Password string `json:"pswd"`
}

type LoginRequest struct {
    Login   string `json:"login"`
    Password string `json:"pswd"`
}

func NewAuthHandler(tokenService *service.TokenService, userService *service.UserService, adminToken string) *AuthHandler {
	return &AuthHandler{
		tokenService: tokenService,
		userService: userService,
		adminToken: adminToken,
	}
}

func (h *AuthHandler) Registration(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

	var req RegistrationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token != h.adminToken {
		http.Error(w, "Invalid admin token", http.StatusForbidden)
		return
	}

    loginRegex := regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)
    if !loginRegex.MatchString(req.Login) {
        http.Error(w, "Invalid login format", http.StatusBadRequest)
        return
    }

    if !h.userService.IsValidPassword(req.Password) {
        http.Error(w, "Invalid password format", http.StatusBadRequest)
        return
    }

	err = h.userService.CreateUser(req.Login, req.Password, r.Context())
	if err != nil {
		http.Error(w, "User creating error", http.StatusInternalServerError)
		return
	}


	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]string{"login": req.Login},
	})
}



func (h *AuthHandler) DeAuthorization(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
        w.Header().Set("Allow", "DELETE")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

	vars := mux.Vars(r)
    token := vars["token"]

    // Проверяем токен
    _, err := h.tokenService.VerifyAccessToken(token, r.Context())
    if err != nil {
        log.Printf("Token verification failed: %v", err)
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

	err = h.tokenService.RevokeToken(r.Context(), token)
	if err != nil {
        log.Printf("Token verification failed: %v", err)
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }
	
    response := map[string]interface{}{
        "response": map[string]bool{
            token: true,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}




func (h *AuthHandler) Authorization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user_id, err := h.userService.VeriefyUser(req.Login, req.Password, r.Context())
	if err != nil {
			http.Error(w, "User don`t exists", http.StatusBadRequest)
			return
		}
	
	token, err := h.tokenService.GenerateAccessToken(req.Login, r.RemoteAddr, user_id, r.Context())
	if err != nil {
			http.Error(w, "Generating token error", http.StatusInternalServerError)
			return
		}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]string{"token": token},
	})
}