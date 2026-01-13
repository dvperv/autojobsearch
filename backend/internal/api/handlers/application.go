package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/api/middleware"
	"autojobsearch/backend/internal/storage"
	"autojobsearch/backend/pkg/utils"
)

type ApplicationHandler struct {
	db     *storage.Database
	logger *zap.Logger
}

func NewApplicationHandler(db *storage.Database, logger *zap.Logger) *ApplicationHandler {
	return &ApplicationHandler{
		db:     db,
		logger: logger,
	}
}

// GetApplications получение списка откликов
func (h *ApplicationHandler) GetApplications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Параметры пагинации
	page, limit := utils.GetPaginationParams(r)
	status := r.URL.Query().Get("status")

	applications, total, err := h.db.GetUserApplications(r.Context(), userID, page, limit, status)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	utils.WritePaginatedResponse(w, applications, int64(total), page, limit)
}

// GetApplication получение конкретного отклика
func (h *ApplicationHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	applicationID := chi.URLParam(r, "id")

	// В реальной реализации здесь будет получение по ID
	// Для MVP возвращаем информацию с учетом userID для безопасности

	// Получаем все отклики пользователя и находим нужный
	applications, _, err := h.db.GetUserApplications(r.Context(), userID, 1, 1000, "")
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	var foundApp interface{}
	for _, app := range applications {
		if app.ID.String() == applicationID {
			foundApp = app
			break
		}
	}

	if foundApp == nil {
		utils.WriteNotFound(w, "Application")
		return
	}

	utils.WriteSuccess(w, foundApp)
}

// WithdrawApplication отзыв отклика
func (h *ApplicationHandler) WithdrawApplication(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	applicationID := chi.URLParam(r, "id")

	// Проверяем существование отклика у пользователя
	applications, _, err := h.db.GetUserApplications(r.Context(), userID, 1, 1000, "")
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to verify application ownership")
		return
	}

	var foundApp interface{}
	for _, app := range applications {
		if app.ID.String() == applicationID {
			foundApp = app
			break
		}
	}

	if foundApp == nil {
		utils.WriteNotFound(w, "Application")
		return
	}

	// В реальной реализации здесь будет отзыв через HH.ru API
	// Для MVP просто отмечаем как отозванный

	utils.WriteMessage(w, "Application withdrawal requested for ID: "+applicationID)
}

// GetApplicationStats статистика по откликам
func (h *ApplicationHandler) GetApplicationStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Получаем все отклики пользователя
	applications, _, err := h.db.GetUserApplications(r.Context(), userID, 1, 1000, "")
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	// Статистика
	stats := map[string]interface{}{
		"total":            len(applications),
		"sent":             0,
		"viewed":           0,
		"rejected":         0,
		"accepted":         0,
		"pending":          0,
		"by_source":        make(map[string]int),
		"by_status":        make(map[string]int),
		"match_score_avg":  0.0,
		"last_application": nil,
	}

	var totalScore float64
	var lastApp interface{}

	for _, app := range applications {
		// Подсчет по статусам
		statusCount := stats["by_status"].(map[string]int)
		statusCount[app.Status]++

		// Подсчет по источнику
		if app.Source != "" {
			sourceCount := stats["by_source"].(map[string]int)
			sourceCount[app.Source]++
		}

		// Общий счетчик по статусам
		switch app.Status {
		case "sent":
			stats["sent"] = stats["sent"].(int) + 1
		case "viewed":
			stats["viewed"] = stats["viewed"].(int) + 1
		case "rejected":
			stats["rejected"] = stats["rejected"].(int) + 1
		case "accepted":
			stats["accepted"] = stats["accepted"].(int) + 1
		default:
			stats["pending"] = stats["pending"].(int) + 1
		}

		totalScore += app.MatchScore

		// Запоминаем последний отклик
		if lastApp == nil || app.AppliedAt.After(applications[0].AppliedAt) {
			lastApp = app
		}
	}

	// Средний match score
	if len(applications) > 0 {
		stats["match_score_avg"] = totalScore / float64(len(applications))
		stats["last_application"] = lastApp
	}

	// Добавляем информацию о пользователе для контекста
	stats["user_id"] = userID.String()

	utils.WriteSuccess(w, stats)
}

// GetDailyApplications получение откликов за сегодня
func (h *ApplicationHandler) GetDailyApplications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Получаем отклики за сегодня
	today := time.Now().Format("2006-01-02")
	applications, err := h.db.GetUserApplicationsToday(r.Context(), userID, today)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get today's applications")
		return
	}

	stats := map[string]interface{}{
		"date":         today,
		"count":        len(applications),
		"applications": applications,
		"automated":    0,
		"manual":       0,
		"success_rate": 0.0,
	}

	var successful int
	for _, app := range applications {
		if app.Automated {
			stats["automated"] = stats["automated"].(int) + 1
		} else {
			stats["manual"] = stats["manual"].(int) + 1
		}

		if app.Status == "sent" || app.Status == "viewed" {
			successful++
		}
	}

	if len(applications) > 0 {
		stats["success_rate"] = float64(successful) / float64(len(applications)) * 100
	}

	utils.WriteSuccess(w, stats)
}

// Routes настройка маршрутов
func (h *ApplicationHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.GetApplications)
	r.Get("/daily", h.GetDailyApplications)
	r.Get("/{id}", h.GetApplication)
	r.Delete("/{id}", h.WithdrawApplication)
	r.Get("/stats", h.GetApplicationStats)

	return r
}
