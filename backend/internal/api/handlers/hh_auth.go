package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"autojobsearch/internal/api/middleware"
	"autojobsearch/internal/services"
	"autojobsearch/pkg/utils"
)

type HHAuthHandler struct {
	hhService *services.HHService
}

func NewHHAuthHandler(hhService *services.HHService) *HHAuthHandler {
	return &HHAuthHandler{hhService: hhService}
}

// ConnectHHAccountRequest запрос на подключение HH.ru
type ConnectHHAccountRequest struct {
	AuthorizationCode string `json:"authorization_code"`
}

// ConnectHHAccount подключение аккаунта HH.ru
func (h *HHAuthHandler) ConnectHHAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	var req ConnectHHAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.AuthorizationCode == "" {
		utils.WriteError(w, http.StatusBadRequest, "Authorization code is required")
		return
	}

	// Обмен кода на токены
	tokens, err := h.hhService.ExchangeCode(r.Context(), userID, req.AuthorizationCode)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Failed to connect HH.ru account: "+err.Error())
		return
	}

	// Получение информации о пользователе
	userInfo, err := h.hhService.GetUserInfo(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get user info: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "HH.ru account connected successfully",
		"user_info":        userInfo,
		"tokens_expire_at": tokens.ExpiresAt,
	})
}

// GetHHAuthURL получение URL для авторизации в HH.ru
func (h *HHAuthHandler) GetHHAuthURL(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Генерация state для защиты от CSRF
	state := uuid.New().String()

	// Сохранение state в сессии или Redis
	sessionKey := fmt.Sprintf("hh_auth_state:%s", userID.String())
	h.redis.SetWithExpiry(r.Context(), sessionKey, state, 10*time.Minute)

	// Получение URL авторизации
	authURL := h.hhService.GetAuthorizationURL(userID, state)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"auth_url": authURL,
		"state":    state,
	})
}

// GetHHStatus получение статуса подключения HH.ru
func (h *HHAuthHandler) GetHHStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	tokens, err := h.hhService.GetOrRefreshTokens(r.Context(), userID)
	if err != nil {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
			"message":   "HH.ru account not connected",
		})
		return
	}

	userInfo, _ := h.hhService.GetUserInfo(r.Context(), userID)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"connected":    true,
		"expires_at":   tokens.ExpiresAt,
		"is_expired":   tokens.IsExpired(),
		"user_info":    userInfo,
		"minutes_left": int(time.Until(tokens.ExpiresAt).Minutes()),
	})
}

// DisconnectHHAccount отключение аккаунта HH.ru
func (h *HHAuthHandler) DisconnectHHAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Удаление токенов из БД
	if err := h.db.DeleteHHTokens(r.Context(), userID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to disconnect HH.ru account")
		return
	}

	// Очистка кэша
	h.hhService.ClearTokenCache(userID)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "HH.ru account disconnected successfully",
	})
}

// Routes настройка маршрутов
func (h *HHAuthHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.AuthMiddleware)

	r.Get("/auth-url", h.GetHHAuthURL)
	r.Post("/connect", h.ConnectHHAccount)
	r.Get("/status", h.GetHHStatus)
	r.Post("/disconnect", h.DisconnectHHAccount)

	return r
}
