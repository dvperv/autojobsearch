package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"autojobsearch/internal/api/middleware"
	"autojobsearch/internal/models"
	"autojobsearch/internal/storage"
	"autojobsearch/pkg/utils"
)

type AuthHandler struct {
	db     *storage.Database
	redis  *storage.RedisClient
	logger *zap.Logger
}

func NewAuthHandler(db *storage.Database, redis *storage.RedisClient, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    time.Time   `json:"expires_at"`
	User         models.User `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Проверка email
	existingUser, _ := h.db.GetUserByEmail(r.Context(), req.Email)
	if existingUser != nil {
		utils.WriteError(w, http.StatusConflict, "User already exists")
		return
	}

	// Создание пользователя (в реальной реализации нужен хэш пароля)
	user := &models.User{
		ID:        uuid.New(),
		Email:     req.Email,
		Password:  req.Password, // В реальности нужно хэшировать
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.CreateUser(r.Context(), user); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Генерация токенов
	accessToken, err := middleware.GenerateJWTToken(user.ID, user.Email, user.FirstName, user.LastName)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken := uuid.New().String()

	// Сохранение refresh token в Redis
	key := fmt.Sprintf("refresh_token:%s", user.ID.String())
	h.redis.SetWithExpiry(r.Context(), key, refreshToken, 7*24*time.Hour)

	response := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		User:         *user,
	}

	utils.WriteSuccess(w, response)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Получение пользователя
	user, err := h.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Проверка пароля (в реальности нужно сравнивать хэши)
	if user.Password != req.Password {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Генерация токенов
	accessToken, err := middleware.GenerateJWTToken(user.ID, user.Email, user.FirstName, user.LastName)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken := uuid.New().String()

	// Сохранение refresh token
	key := fmt.Sprintf("refresh_token:%s", user.ID.String())
	h.redis.SetWithExpiry(r.Context(), key, refreshToken, 7*24*time.Hour)

	response := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		User:         *user,
	}

	utils.WriteSuccess(w, response)
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Реализация обновления токена
	utils.WriteMessage(w, "Token refresh endpoint")
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Удаление refresh token из Redis
	key := fmt.Sprintf("refresh_token:%s", userID.String())
	h.redis.Delete(r.Context(), key)

	utils.WriteMessage(w, "Logged out successfully")
}

func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	// Не возвращаем пароль
	user.Password = ""
	utils.WriteSuccess(w, user)
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	// Обновление полей
	if firstName, ok := updates["first_name"].(string); ok {
		user.FirstName = firstName
	}
	if lastName, ok := updates["last_name"].(string); ok {
		user.LastName = lastName
	}
	if phone, ok := updates["phone"].(string); ok {
		user.Phone = &phone
	}

	user.UpdatedAt = time.Now()

	if err := h.db.UpdateUser(r.Context(), user); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	utils.WriteSuccess(w, user)
}
