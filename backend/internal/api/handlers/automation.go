package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"autojobsearch/internal/api/middleware"
	"autojobsearch/internal/services"
	"autojobsearch/pkg/utils"
)

type AutomationHandler struct {
	automationService *services.AutomationEngine
	userService       *services.UserService
}

func NewAutomationHandler(
	automationService *services.AutomationEngine,
	userService *services.UserService,
) *AutomationHandler {
	return &AutomationHandler{
		automationService: automationService,
		userService:       userService,
	}
}

// StartAutomationRequest запрос на запуск автоматизации
type StartAutomationRequest struct {
	Schedule struct {
		Enabled    bool   `json:"enabled"`
		Frequency  string `json:"frequency"`   // daily, weekly
		TimeOfDay  string `json:"time_of_day"` // HH:MM
		DaysOfWeek []int  `json:"days_of_week"`
	} `json:"schedule"`
	Settings struct {
		Positions  []string `json:"positions"`
		SalaryMin  int      `json:"salary_min"`
		SalaryMax  int      `json:"salary_max"`
		Locations  []string `json:"locations"`
		Experience string   `json:"experience"`
		Employment string   `json:"employment"`
		Schedule   string   `json:"schedule"`
	} `json:"settings"`
}

// StartAutomation запуск автоматического поиска и откликов
func (h *AutomationHandler) StartAutomation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	var req StartAutomationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Преобразование в модель SearchSettings
	settings := models.SearchSettings{
		UserID:     userID,
		Positions:  req.Settings.Positions,
		SalaryMin:  req.Settings.SalaryMin,
		SalaryMax:  req.Settings.SalaryMax,
		Locations:  req.Settings.Locations,
		Experience: req.Settings.Experience,
		Employment: req.Settings.Employment,
		Schedule:   req.Settings.Schedule,
	}

	// Сохранение настроек
	if err := h.userService.SaveSearchSettings(r.Context(), userID, settings); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save settings")
		return
	}

	// Запуск автоматизации
	job, err := h.automationService.StartAutomation(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to start automation: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusCreated, job)
}

// StopAutomation остановка автоматизации
func (h *AutomationHandler) StopAutomation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	if err := h.automationService.StopAutomation(r.Context(), userID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to stop automation: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Automation stopped successfully",
	})
}

// GetAutomationStatus получение статуса автоматизации
func (h *AutomationHandler) GetAutomationStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	status, err := h.automationService.GetAutomationStatus(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Automation not found")
		return
	}

	utils.WriteJSON(w, http.StatusOK, status)
}

// GetAutomationStats получение статистики автоматизации
func (h *AutomationHandler) GetAutomationStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	stats, err := h.userService.GetAutomationStats(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	utils.WriteJSON(w, http.StatusOK, stats)
}

// GetApplications получение списка отправленных откликов
func (h *AutomationHandler) GetApplications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Параметры запроса
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	status := r.URL.Query().Get("status")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	applications, total, err := h.userService.GetApplications(r.Context(), userID, page, limit, status)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"applications": applications,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"pages":        int(math.Ceil(float64(total) / float64(limit))),
	})
}

// GetInvitations получение приглашений на собеседования
func (h *AutomationHandler) GetInvitations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	invitations, err := h.userService.GetInvitations(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get invitations")
		return
	}

	utils.WriteJSON(w, http.StatusOK, invitations)
}

// UpdateAutomationSettings обновление настроек автоматизации
func (h *AutomationHandler) UpdateAutomationSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Разрешенные поля для обновления
	allowedFields := map[string]bool{
		"schedule.enabled":      true,
		"schedule.frequency":    true,
		"schedule.time_of_day":  true,
		"schedule.days_of_week": true,
		"settings.positions":    true,
		"settings.salary_min":   true,
		"settings.salary_max":   true,
		"settings.locations":    true,
		"settings.experience":   true,
		"settings.employment":   true,
		"settings.schedule":     true,
	}

	// Фильтрация полей
	filteredUpdates := make(map[string]interface{})
	for key, value := range updates {
		if allowedFields[key] {
			filteredUpdates[key] = value
		}
	}

	if err := h.userService.UpdateAutomationSettings(r.Context(), userID, filteredUpdates); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Settings updated successfully",
	})
}

// RunAutomationNow немедленный запуск автоматизации
func (h *AutomationHandler) RunAutomationNow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Проверка наличия активной автоматизации
	status, err := h.automationService.GetAutomationStatus(r.Context(), userID)
	if err != nil || status.Status != "active" {
		utils.WriteError(w, http.StatusBadRequest, "Automation is not active")
		return
	}

	// Запуск немедленного поиска
	go func() {
		ctx := context.Background()
		if _, err := h.automationService.performAutomatedSearch(ctx, &services.AutomationJob{
			ID:     status.JobID,
			UserID: userID,
		}); err != nil {
			h.logger.Error("Failed to run automation now",
				zap.String("user_id", userID.String()),
				zap.Error(err))
		}
	}()

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Automation started immediately",
	})
}

// Routes настройка маршрутов
func (h *AutomationHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.AuthMiddleware)

	r.Post("/start", h.StartAutomation)
	r.Post("/stop", h.StopAutomation)
	r.Get("/status", h.GetAutomationStatus)
	r.Get("/stats", h.GetAutomationStats)
	r.Get("/applications", h.GetApplications)
	r.Get("/invitations", h.GetInvitations)
	r.Put("/settings", h.UpdateAutomationSettings)
	r.Post("/run-now", h.RunAutomationNow)

	return r
}
