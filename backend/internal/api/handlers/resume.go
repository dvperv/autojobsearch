package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/api/middleware"
	"autojobsearch/backend/internal/models"
	"autojobsearch/backend/internal/storage"
	"autojobsearch/backend/pkg/utils"
)

type ResumeHandler struct {
	db     *storage.Database
	logger *zap.Logger
}

func NewResumeHandler(db *storage.Database, logger *zap.Logger) *ResumeHandler {
	return &ResumeHandler{
		db:     db,
		logger: logger,
	}
}

// UploadResume загрузка резюме
func (h *ResumeHandler) UploadResume(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	// Обработка multipart/form-data
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("resume")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	// Проверка типа файла
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExtensions := map[string]bool{
		".pdf":  true,
		".doc":  true,
		".docx": true,
		".txt":  true,
	}

	if !allowedExtensions[ext] {
		utils.WriteError(w, http.StatusBadRequest, "Unsupported file type")
		return
	}

	// Здесь будет парсинг резюме
	// Для MVP просто сохраняем информацию о файле

	resume := &models.Resume{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     strings.TrimSuffix(header.Filename, ext),
		FilePath:  "/uploads/" + header.Filename,
		FileType:  ext[1:], // без точки
		FileSize:  header.Size,
		IsPrimary: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.SaveResume(r.Context(), resume); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save resume")
		return
	}

	utils.WriteSuccess(w, resume)
}

// GetResumes получение списка резюме
func (h *ResumeHandler) GetResumes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())

	resumes, err := h.db.GetUserResumes(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get resumes")
		return
	}

	utils.WriteSuccess(w, resumes)
}

// DeleteResume удаление резюме
func (h *ResumeHandler) DeleteResume(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	resumeIDStr := chi.URLParam(r, "id")

	resumeID, err := uuid.Parse(resumeIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid resume ID")
		return
	}

	// Проверяем, что резюме принадлежит пользователю
	resumes, err := h.db.GetUserResumes(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to verify resume ownership")
		return
	}

	found := false
	for _, resume := range resumes {
		if resume.ID == resumeID {
			found = true
			break
		}
	}

	if !found {
		utils.WriteError(w, http.StatusNotFound, "Resume not found")
		return
	}

	if err := h.db.DeleteResume(r.Context(), resumeID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete resume")
		return
	}

	utils.WriteMessage(w, "Resume deleted successfully")
}

// SetPrimaryResume установка основного резюме
func (h *ResumeHandler) SetPrimaryResume(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	resumeIDStr := chi.URLParam(r, "id")

	resumeID, err := uuid.Parse(resumeIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid resume ID")
		return
	}

	// Получаем все резюме пользователя
	resumes, err := h.db.GetUserResumes(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get resumes")
		return
	}

	// Сбрасываем isPrimary у всех резюме и устанавливаем для выбранного
	for _, resume := range resumes {
		resume.IsPrimary = (resume.ID == resumeID)
		if err := h.db.UpdateResume(r.Context(), &resume); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Failed to update resume")
			return
		}
	}

	utils.WriteMessage(w, "Primary resume updated")
}

// Routes настройка маршрутов
func (h *ResumeHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.GetResumes)
	r.Post("/upload", h.UploadResume)
	r.Delete("/{id}", h.DeleteResume)
	r.Put("/{id}/primary", h.SetPrimaryResume)

	return r
}
