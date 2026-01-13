package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// JSONResponse стандартный JSON ответ
type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// WriteJSON записывает JSON ответ
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// WriteSuccess успешный ответ
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	response := JSONResponse{
		Success: true,
		Data:    data,
	}
	WriteJSON(w, http.StatusOK, response)
}

// WriteError ответ с ошибкой
func WriteError(w http.ResponseWriter, status int, message string) {
	response := JSONResponse{
		Success: false,
		Error:   message,
	}
	WriteJSON(w, status, response)
}

// WriteMessage ответ с сообщением
func WriteMessage(w http.ResponseWriter, message string) {
	response := JSONResponse{
		Success: true,
		Message: message,
	}
	WriteJSON(w, http.StatusOK, response)
}

// WriteValidationError ошибка валидации
func WriteValidationError(w http.ResponseWriter, errors map[string]string) {
	response := map[string]interface{}{
		"success": false,
		"error":   "Validation failed",
		"errors":  errors,
	}
	WriteJSON(w, http.StatusBadRequest, response)
}

// WriteNotFound 404 ошибка
func WriteNotFound(w http.ResponseWriter, resource string) {
	WriteError(w, http.StatusNotFound, resource+" not found")
}

// WriteUnauthorized 401 ошибка
func WriteUnauthorized(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, "Unauthorized")
}

// WriteForbidden 403 ошибка
func WriteForbidden(w http.ResponseWriter) {
	WriteError(w, http.StatusForbidden, "Forbidden")
}

// WriteInternalError 500 ошибка
func WriteInternalError(w http.ResponseWriter, err error) {
	// В продакшене не показываем детали ошибки
	WriteError(w, http.StatusInternalServerError, "Internal server error")

	// Логируем ошибку
	// logger.Error("Internal server error", zap.Error(err))
}

// PaginatedResponse пагинированный ответ
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// WritePaginatedResponse пагинированный ответ
func WritePaginatedResponse(w http.ResponseWriter, data interface{}, total int64, page, limit int) {
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	response := PaginatedResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	WriteSuccess(w, response)
}

// HealthCheckResponse ответ для health check
type HealthCheckResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
}

// WriteHealthCheck записывает health check ответ
func WriteHealthCheck(w http.ResponseWriter, status string, services map[string]string) {
	response := HealthCheckResponse{
		Status:    status,
		Timestamp: time.Now(),
		Services:  services,
	}
	WriteJSON(w, http.StatusOK, response)
}

// BindJSON парсит JSON из тела запроса
func BindJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}

	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// GetQueryParam получает параметр из query string
func GetQueryParam(r *http.Request, key string, defaultValue string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetQueryInt получает int параметр из query string
func GetQueryInt(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// GetPaginationParams получает параметры пагинации
func GetPaginationParams(r *http.Request) (page, limit int) {
	page = GetQueryInt(r, "page", 1)
	limit = GetQueryInt(r, "limit", 20)

	// Ограничения
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	return page, limit
}
