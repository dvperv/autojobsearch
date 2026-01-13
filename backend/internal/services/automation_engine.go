package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"autojobsearch/internal/models"
	"autojobsearch/internal/storage"
)

// AutomationEngine - основной движок автоматизации
type AutomationEngine struct {
	db          *storage.Database
	redis       *storage.RedisClient
	hhService   *HHService
	matcher     *SmartMatcher
	notifier    *NotificationService
	logger      *zap.Logger
	cron        *cron.Cron
	runningJobs sync.Map
	config      AutomationConfig
	stats       *AutomationStats
	mu          sync.RWMutex
}

type AutomationConfig struct {
	SearchInterval        time.Duration `json:"search_interval"`           // 24 часа
	MaxDailySearches      int           `json:"max_daily_searches"`        // 1
	MaxDailyApplications  int           `json:"max_daily_applications"`    // 50
	MinMatchScore         float64       `json:"min_match_score"`           // 0.7
	ApplyImmediately      bool          `json:"apply_immediately"`         // true для MVP
	RetryAttempts         int           `json:"retry_attempts"`            // 3
	MaxAPIRequestsPerHour int           `json:"max_api_requests_per_hour"` // 500 (HH.ru лимит)
}

type AutomationJob struct {
	ID         uuid.UUID             `json:"id"`
	UserID     uuid.UUID             `json:"user_id"`
	Schedule   AutomationSchedule    `json:"schedule"`
	Settings   models.SearchSettings `json:"settings"`
	Status     string                `json:"status"` // active, paused, completed, hh_disconnected
	Statistics JobStatistics         `json:"statistics"`
	LastRun    *time.Time            `json:"last_run,omitempty"`
	NextRun    *time.Time            `json:"next_run,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`

	// Флаги состояния
	HHConnected bool   `json:"hh_connected"`
	LastError   string `json:"last_error,omitempty"`
}

type AutomationSchedule struct {
	Enabled    bool   `json:"enabled"`
	Frequency  string `json:"frequency"`    // daily, weekly, manual
	TimeOfDay  string `json:"time_of_day"`  // HH:MM format, e.g., "08:00"
	DaysOfWeek []int  `json:"days_of_week"` // 0-6, где 0 = воскресенье
}

type JobStatistics struct {
	TotalRuns           int        `json:"total_runs"`
	VacanciesFound      int        `json:"vacancies_found"`
	ApplicationsSent    int        `json:"applications_sent"`
	InvitationsReceived int        `json:"invitations_received"`
	AvgMatchScore       float64    `json:"avg_match_score"`
	SuccessRate         float64    `json:"success_rate"`
	HHRequestsCount     int        `json:"hh_requests_count"`
	LastHHRequestAt     *time.Time `json:"last_hh_request_at,omitempty"`
}

type AutomationStats struct {
	ActiveJobs        int     `json:"active_jobs"`
	TotalUsers        int     `json:"total_users"`
	ApplicationsToday int     `json:"applications_today"`
	InvitationsToday  int     `json:"invitations_today"`
	AvgResponseTime   float64 `json:"avg_response_time"`
	TotalHHRequests   int     `json:"total_hh_requests"`
	HHRateLimitUsage  float64 `json:"hh_rate_limit_usage"` // процент использования лимита
}

func NewAutomationEngine(
	db *storage.Database,
	redis *storage.RedisClient,
	hhService *HHService,
	matcher *SmartMatcher,
	notifier *NotificationService,
	logger *zap.Logger,
) *AutomationEngine {
	config := AutomationConfig{
		SearchInterval:        24 * time.Hour,
		MaxDailySearches:      1,
		MaxDailyApplications:  50,
		MinMatchScore:         0.7,
		ApplyImmediately:      true,
		RetryAttempts:         3,
		MaxAPIRequestsPerHour: 500, // HH.ru лимит на пользователя
	}

	return &AutomationEngine{
		db:        db,
		redis:     redis,
		hhService: hhService,
		matcher:   matcher,
		notifier:  notifier,
		logger:    logger,
		cron:      cron.New(cron.WithSeconds()),
		config:    config,
		stats:     &AutomationStats{},
	}
}

// StartAutomation - запуск автоматизации для пользователя
func (e *AutomationEngine) StartAutomation(ctx context.Context, userID uuid.UUID) (*AutomationJob, error) {
	// Проверка подключения HH.ru
	hhConnected, err := e.checkHHConnection(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check HH.ru connection: %w", err)
	}

	if !hhConnected {
		return nil, fmt.Errorf("HH.ru account not connected")
	}

	// Проверка, есть ли уже активная автоматизация
	existingJob, err := e.db.GetUserAutomationJob(ctx, userID)
	if err == nil && existingJob != nil {
		if existingJob.Status == "active" {
			return existingJob, fmt.Errorf("automation already running for user")
		}

		// Если задание существует, но неактивно, обновляем его
		existingJob.Status = "active"
		existingJob.UpdatedAt = time.Now()
		existingJob.NextRun = e.calculateNextRun(time.Now(), existingJob.Schedule.TimeOfDay)

		if err := e.db.UpdateAutomationJob(ctx, existingJob); err != nil {
			return nil, fmt.Errorf("failed to update automation job: %w", err)
		}

		// Запуск планировщика
		if err := e.scheduleJob(existingJob); err != nil {
			return nil, fmt.Errorf("failed to schedule job: %w", err)
		}

		return existingJob, nil
	}

	// Получение настроек пользователя
	settings, err := e.db.GetUserSearchSettings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user search settings not found: %w", err)
	}

	// Проверка наличия резюме
	resumes, err := e.db.GetUserResumes(ctx, userID)
	if err != nil || len(resumes) == 0 {
		// Для HH.ru используем резюме из аккаунта HH.ru, а не локальные
		hhResumes, hhErr := e.hhService.GetUserResumes(ctx, userID)
		if hhErr != nil || len(hhResumes) == 0 {
			return nil, fmt.Errorf("no resumes found in HH.ru account")
		}
	}

	// Создание задания автоматизации
	job := &AutomationJob{
		ID:     uuid.New(),
		UserID: userID,
		Schedule: AutomationSchedule{
			Enabled:    true,
			Frequency:  "daily",
			TimeOfDay:  "08:00",
			DaysOfWeek: []int{1, 2, 3, 4, 5}, // Пн-Пт
		},
		Settings:    settings,
		Status:      "active",
		Statistics:  JobStatistics{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		HHConnected: hhConnected,
	}

	nextRun := e.calculateNextRun(time.Now(), job.Schedule.TimeOfDay)
	job.NextRun = &nextRun

	// Сохранение в БД
	if err := e.db.SaveAutomationJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save automation job: %w", err)
	}

	// Запуск планировщика
	if err := e.scheduleJob(job); err != nil {
		return nil, fmt.Errorf("failed to schedule job: %w", err)
	}

	// Немедленный запуск первого поиска (для MVP)
	go e.executeJobImmediately(job)

	// Отправка уведомления пользователю
	e.notifier.SendAutomationStarted(userID, job)

	e.logger.Info("Automation started",
		zap.String("user_id", userID.String()),
		zap.String("job_id", job.ID.String()),
		zap.Bool("hh_connected", hhConnected))

	return job, nil
}

// checkHHConnection - проверка подключения HH.ru аккаунта
func (e *AutomationEngine) checkHHConnection(ctx context.Context, userID uuid.UUID) (bool, error) {
	// Проверяем наличие токенов в БД
	tokens, err := e.db.GetHHTokens(ctx, userID)
	if err != nil {
		return false, nil // Нет токенов - не подключен
	}

	// Проверяем срок действия токенов
	if tokens.IsExpired() {
		// Пытаемся обновить токены
		_, refreshErr := e.hhService.refreshTokens(ctx, tokens)
		if refreshErr != nil {
			e.logger.Warn("HH.ru tokens expired and refresh failed",
				zap.String("user_id", userID.String()),
				zap.Error(refreshErr))
			return false, nil
		}
	}

	return true, nil
}

// executeJobImmediately - немедленное выполнение задания
func (e *AutomationEngine) executeJobImmediately(job *AutomationJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	e.logger.Info("Executing automation job immediately",
		zap.String("user_id", job.UserID.String()),
		zap.String("job_id", job.ID.String()))

	// Выполнение автоматического поиска и откликов
	result, err := e.performAutomatedSearch(ctx, job)
	if err != nil {
		e.logger.Error("Failed to execute automation job",
			zap.String("user_id", job.UserID.String()),
			zap.String("job_id", job.ID.String()),
			zap.Error(err))

		// Обновляем статус задания с ошибкой
		job.LastError = err.Error()
		job.UpdatedAt = time.Now()

		if strings.Contains(err.Error(), "HH.ru") || strings.Contains(err.Error(), "token") {
			job.HHConnected = false
			job.Status = "hh_disconnected"
			e.notifier.SendHHConnectionLost(job.UserID)
		}

		if updateErr := e.db.UpdateAutomationJob(ctx, job); updateErr != nil {
			e.logger.Error("Failed to update automation job after error",
				zap.String("job_id", job.ID.String()),
				zap.Error(updateErr))
		}
		return
	}

	// Обновление статистики
	job.Statistics.TotalRuns++
	job.Statistics.VacanciesFound += result.VacanciesFound
	job.Statistics.ApplicationsSent += result.ApplicationsSent
	job.Statistics.HHRequestsCount += result.HHRequestsCount

	now := time.Now()
	job.LastRun = &now
	nextRun := e.calculateNextRun(time.Now(), job.Schedule.TimeOfDay)
	job.NextRun = &nextRun
	job.LastError = ""
	job.HHConnected = true

	// Расчет среднего score
	if result.ApplicationsSent > 0 {
		totalScore := job.Statistics.AvgMatchScore * float64(job.Statistics.TotalRuns-1)
		totalScore += result.AvgMatchScore
		job.Statistics.AvgMatchScore = totalScore / float64(job.Statistics.TotalRuns)
	}

	// Сохранение обновлений
	if err := e.db.UpdateAutomationJob(ctx, job); err != nil {
		e.logger.Error("Failed to update automation job",
			zap.String("job_id", job.ID.String()),
			zap.Error(err))
	}

	// Обновление глобальной статистики
	e.updateGlobalStats(result)
}

// performAutomatedSearch - выполнение автоматического поиска и откликов
func (e *AutomationEngine) performAutomatedSearch(ctx context.Context, job *AutomationJob) (*AutomationResult, error) {
	result := &AutomationResult{
		JobID:           job.ID,
		UserID:          job.UserID,
		StartedAt:       time.Now(),
		HHRequestsCount: 0,
	}

	// 1. Проверка подключения HH.ru
	hhConnected, err := e.checkHHConnection(ctx, job.UserID)
	if err != nil || !hhConnected {
		return nil, fmt.Errorf("HH.ru account not connected or tokens invalid")
	}

	// 2. Получение резюме пользователя из HH.ru
	hhResumes, err := e.hhService.GetUserResumes(ctx, job.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user resumes from HH.ru: %w", err)
	}

	if len(hhResumes) == 0 {
		return nil, fmt.Errorf("no resumes found in HH.ru account")
	}

	// Используем основное резюме (первое в списке)
	primaryResume := hhResumes[0]
	result.HHRequestsCount++

	// 3. Поиск вакансий через HH.ru
	vacancies, err := e.searchVacancies(ctx, job.UserID, job.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to search vacancies: %w", err)
	}

	result.HHRequestsCount++
	result.VacanciesFound = len(vacancies)

	if len(vacancies) == 0 {
		result.CompletedAt = time.Now()
		result.Success = true
		return result, nil
	}

	// 4. Фильтрация новых вакансий
	newVacancies := e.filterNewVacancies(ctx, job.UserID, vacancies)
	result.NewVacancies = len(newVacancies)

	if len(newVacancies) == 0 {
		result.CompletedAt = time.Now()
		result.Success = true
		return result, nil
	}

	// 5. Обработка каждой новой вакансии
	var applications []*models.Application
	var totalMatchScore float64

	for _, vacancy := range newVacancies {
		// Проверка соответствия
		matchResult, err := e.matcher.MatchVacancy(ctx, vacancy, primaryResume)
		if err != nil {
			e.logger.Warn("Failed to match vacancy",
				zap.String("vacancy_id", vacancy.ID),
				zap.String("user_id", job.UserID.String()),
				zap.Error(err))
			continue
		}

		totalMatchScore += matchResult.Score

		// Проверка порога соответствия
		if matchResult.Score >= e.config.MinMatchScore {
			// Автоматический отклик
			application, err := e.applyAutomatically(ctx, job.UserID, vacancy, primaryResume, matchResult)
			if err != nil {
				e.logger.Warn("Failed to apply automatically",
					zap.String("vacancy_id", vacancy.ID),
					zap.String("user_id", job.UserID.String()),
					zap.Error(err))
				continue
			}

			applications = append(applications, application)
			result.ApplicationsSent++

			result.HHRequestsCount++

			// Ограничение по количеству откликов в день
			if result.ApplicationsSent >= e.config.MaxDailyApplications {
				e.logger.Info("Daily application limit reached",
					zap.String("user_id", job.UserID.String()),
					zap.Int("limit", e.config.MaxDailyApplications))
				break
			}
		}
	}

	// 6. Сохранение результатов
	if err := e.saveAutomationResults(ctx, job.UserID, applications); err != nil {
		return nil, fmt.Errorf("failed to save results: %w", err)
	}

	// 7. Расчет среднего match score
	if len(newVacancies) > 0 {
		result.AvgMatchScore = totalMatchScore / float64(len(newVacancies))
	}

	// 8. Отправка уведомления пользователю
	e.sendAutomationReport(ctx, job.UserID, result)

	result.CompletedAt = time.Now()
	result.Success = true

	e.logger.Info("Automation search completed",
		zap.String("user_id", job.UserID.String()),
		zap.String("job_id", job.ID.String()),
		zap.Int("vacancies_found", result.VacanciesFound),
		zap.Int("applications_sent", result.ApplicationsSent),
		zap.Int("hh_requests", result.HHRequestsCount))

	return result, nil
}

// applyAutomatically - автоматический отклик на вакансию через HH.ru API
func (e *AutomationEngine) applyAutomatically(
	ctx context.Context,
	userID uuid.UUID,
	vacancy models.HHVacancy,
	resume models.HHResume,
	matchResult *MatchResult,
) (*models.Application, error) {
	// Проверка rate limit для HH.ru API
	allowed, waitTime, err := e.hhService.CheckRateLimit(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check HH.ru rate limit: %w", err)
	}

	if !allowed {
		return nil, fmt.Errorf("HH.ru rate limit exceeded, wait %v", waitTime)
	}

	// 1. Генерация сопроводительного письма
	coverLetter := e.generateCoverLetter(vacancy, resume, matchResult)

	// 2. Подготовка отклика
	application := &models.Application{
		ID:           uuid.New(),
		UserID:       userID,
		VacancyID:    vacancy.ID,
		VacancyTitle: vacancy.Name,
		CompanyName:  vacancy.Employer.Name,
		ResumeID:     resume.ID,
		CoverLetter:  coverLetter,
		Status:       "pending",
		MatchScore:   matchResult.Score,
		AppliedAt:    time.Now(),
		Automated:    true,
		Source:       "hh.ru",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 3. Отправка через HH.ru API ОТ ИМЕНИ ПОЛЬЗОВАТЕЛЯ
	if err := e.hhService.SendApplication(ctx, userID, vacancy.ID, application); err != nil {
		// Анализ ошибки
		if strings.Contains(err.Error(), "already applied") {
			application.Status = "duplicate"
			application.ErrorMessage = "Already applied to this vacancy on HH.ru"
		} else if strings.Contains(err.Error(), "rate limit") {
			application.Status = "rate_limited"
			application.ErrorMessage = "HH.ru rate limit exceeded"
		} else if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "auth") {
			application.Status = "auth_failed"
			application.ErrorMessage = "HH.ru authentication failed"
		} else {
			application.Status = "failed"
			application.ErrorMessage = err.Error()
		}

		e.logger.Error("Failed to send application via HH.ru",
			zap.String("user_id", userID.String()),
			zap.String("vacancy_id", vacancy.ID),
			zap.String("company", vacancy.Employer.Name),
			zap.String("error", err.Error()))
	} else {
		application.Status = "sent"
		application.HHApplicationID = "pending" // Будет обновлено после получения ID

		e.logger.Info("Auto-application sent via HH.ru",
			zap.String("user_id", userID.String()),
			zap.String("vacancy_id", vacancy.ID),
			zap.String("company", vacancy.Employer.Name),
			zap.Float64("match_score", matchResult.Score))
	}

	// 4. Сохранение в БД
	if err := e.db.SaveApplication(ctx, application); err != nil {
		return nil, fmt.Errorf("failed to save application: %w", err)
	}

	return application, nil
}

// generateCoverLetter - генерация сопроводительного письма
func (e *AutomationEngine) generateCoverLetter(vacancy models.HHVacancy, resume models.HHResume, matchResult *MatchResult) string {
	// Простой шаблон для MVP
	template := `Уважаемая команда %s!

Я, %s, с интересом ознакомился с вакансией "%s" на HH.ru и хотел бы откликнуться на нее.

Мой профиль соответствует вашим требованиям:
%s

Мой опыт работы: %d+ лет
Ключевые навыки: %s

Буду рад обсудить возможность сотрудничества.

С уважением,
%s
%s
`

	// Извлечение релевантных навыков
	relevantSkills := ""
	if len(matchResult.MatchedSkills) > 0 {
		skills := matchResult.MatchedSkills
		if len(skills) > 5 {
			skills = skills[:5]
		}
		relevantSkills = strings.Join(skills, ", ")
	}

	// Опыт работы (упрощенно)
	experienceYears := 0
	if len(resume.Experience) > 0 {
		// Берем первый опыт работы для расчета стажа
		exp := resume.Experience[0]
		if exp.EndDate == "" || exp.EndDate == "present" {
			// Все еще работает
			start, _ := time.Parse("2006-01-02", exp.StartDate)
			experienceYears = int(time.Since(start).Hours() / 24 / 365)
		}
	}

	// Заполнение шаблона
	return fmt.Sprintf(template,
		vacancy.Employer.Name,
		strings.TrimSpace(fmt.Sprintf("%s %s", resume.FirstName, resume.LastName)),
		vacancy.Name,
		e.generateMatchDescription(matchResult),
		experienceYears,
		relevantSkills,
		strings.TrimSpace(fmt.Sprintf("%s %s", resume.FirstName, resume.LastName)),
		resume.Email,
	)
}

// generateMatchDescription - описание соответствия
func (e *AutomationEngine) generateMatchDescription(matchResult *MatchResult) string {
	if matchResult.Score >= 0.9 {
		return "Мой опыт идеально соответствует требованиям вакансии"
	} else if matchResult.Score >= 0.8 {
		return "Мой профиль хорошо соответствует вашим требованиям"
	} else if matchResult.Score >= 0.7 {
		return "Мой опыт частично соответствует требованиям вакансии"
	}
	return "Мой профиль имеет некоторые пересечения с требованиями"
}

// searchVacancies - поиск вакансий с учетом ограничений HH.ru API
func (e *AutomationEngine) searchVacancies(ctx context.Context, userID uuid.UUID, settings models.SearchSettings) ([]models.HHVacancy, error) {
	// Проверка daily limit нашего приложения
	if err := e.checkDailyLimit(ctx, userID); err != nil {
		return nil, err
	}

	// Проверка rate limit для HH.ru API
	allowed, waitTime, err := e.hhService.CheckRateLimit(ctx, userID)
	if err != nil {
		e.logger.Warn("Failed to check HH.ru rate limit",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	if !allowed {
		return nil, fmt.Errorf("HH.ru rate limit exceeded, wait %v", waitTime)
	}

	// Подготовка параметров поиска
	params := map[string]string{
		"text":             strings.Join(settings.Positions, " OR "),
		"area":             settings.AreaID,
		"experience":       e.mapExperience(settings.Experience),
		"employment":       e.mapEmployment(settings.Employment),
		"schedule":         e.mapSchedule(settings.Schedule),
		"salary":           fmt.Sprintf("%d", settings.Salary),
		"order_by":         "publication_time",
		"search_period":    "1", // За последние 24 часа
		"per_page":         "100",
		"page":             "0",
		"only_with_salary": "true",
	}

	// Поиск через HH.ru API ОТ ИМЕНИ ПОЛЬЗОВАТЕЛЯ
	vacancies, err := e.hhService.SearchVacancies(ctx, userID, params)
	if err != nil {
		// Анализ ошибки
		if strings.Contains(err.Error(), "rate limit") {
			// Обновляем rate limit в Redis
			key := fmt.Sprintf("rate_limit:hh:user:%s", userID.String())
			window := time.Hour
			e.redis.SetWithExpiry(ctx, key, fmt.Sprintf("%d", e.config.MaxAPIRequestsPerHour), window)
		}

		return nil, fmt.Errorf("failed to search vacancies via HH.ru: %w", err)
	}

	// Обновление счетчика
	e.updateSearchCounter(ctx, userID)

	// Обновление счетчика HH.ru запросов
	e.updateHHRequestCounter(ctx, userID)

	return vacancies, nil
}

// filterNewVacancies - фильтрация только новых вакансий
func (e *AutomationEngine) filterNewVacancies(ctx context.Context, userID uuid.UUID, vacancies []models.HHVacancy) []models.HHVacancy {
	var newVacancies []models.HHVacancy

	for _, vacancy := range vacancies {
		// Проверка, была ли уже обработана эта вакансия
		processed, err := e.db.IsVacancyProcessed(ctx, userID, vacancy.ID)
		if err != nil {
			e.logger.Warn("Failed to check vacancy processed status",
				zap.String("vacancy_id", vacancy.ID),
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		if !processed {
			newVacancies = append(newVacancies, vacancy)
		}
	}

	return newVacancies
}

// checkDailyLimit - проверка daily limit нашего приложения
func (e *AutomationEngine) checkDailyLimit(ctx context.Context, userID uuid.UUID) error {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("user:%s:searches:%s", userID.String(), today)

	count, err := e.redis.GetInt(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check daily limit: %w", err)
	}

	if count >= e.config.MaxDailySearches {
		return fmt.Errorf("daily search limit reached: %d/%d", count, e.config.MaxDailySearches)
	}

	return nil
}

// updateSearchCounter - обновление счетчика поисков
func (e *AutomationEngine) updateSearchCounter(ctx context.Context, userID uuid.UUID) {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("user:%s:searches:%s", userID.String(), today)

	// Увеличение счетчика
	if err := e.redis.Increment(ctx, key); err != nil {
		e.logger.Error("Failed to increment search counter",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	// Установка TTL до конца дня
	expireAt := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour)
	ttl := expireAt.Sub(time.Now())

	if err := e.redis.Expire(ctx, key, ttl); err != nil {
		e.logger.Error("Failed to set TTL for search counter",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}
}

// updateHHRequestCounter - обновление счетчика HH.ru запросов
func (e *AutomationEngine) updateHHRequestCounter(ctx context.Context, userID uuid.UUID) {
	hour := time.Now().Format("2006-01-02-15")
	key := fmt.Sprintf("hh_requests:user:%s:%s", userID.String(), hour)

	// Увеличение счетчика
	if err := e.redis.Increment(ctx, key); err != nil {
		e.logger.Error("Failed to increment HH.ru request counter",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	// Установка TTL на 1 час
	if err := e.redis.Expire(ctx, key, time.Hour); err != nil {
		e.logger.Error("Failed to set TTL for HH.ru request counter",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}
}

// saveAutomationResults - сохранение результатов автоматизации
func (e *AutomationEngine) saveAutomationResults(ctx context.Context, userID uuid.UUID, applications []*models.Application) error {
	// Сохранение всех откликов
	for _, app := range applications {
		if err := e.db.SaveApplication(ctx, app); err != nil {
			return fmt.Errorf("failed to save application: %w", err)
		}

		// Помечаем вакансию как обработанную
		if err := e.db.MarkVacancyProcessed(ctx, userID, app.VacancyID); err != nil {
			e.logger.Warn("Failed to mark vacancy as processed",
				zap.String("vacancy_id", app.VacancyID),
				zap.String("user_id", userID.String()),
				zap.Error(err))
		}
	}
	return nil
}

// sendAutomationReport - отправка отчета пользователю
func (e *AutomationEngine) sendAutomationReport(ctx context.Context, userID uuid.UUID, result *AutomationResult) {
	// Подготовка отчета
	report := &AutomationReport{
		UserID:           userID,
		JobID:            result.JobID,
		VacanciesFound:   result.VacanciesFound,
		NewVacancies:     result.NewVacancies,
		ApplicationsSent: result.ApplicationsSent,
		AvgMatchScore:    result.AvgMatchScore,
		HHRequestsCount:  result.HHRequestsCount,
		Duration:         result.CompletedAt.Sub(result.StartedAt),
		Timestamp:        time.Now(),
	}

	// Отправка уведомления
	e.notifier.SendAutomationReport(userID, report)
}

// updateGlobalStats - обновление глобальной статистики
func (e *AutomationEngine) updateGlobalStats(result *AutomationResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.ApplicationsToday += result.ApplicationsSent
	e.stats.TotalHHRequests += result.HHRequestsCount

	// Расчет использования rate limit
	if e.stats.TotalHHRequests > 0 {
		e.stats.HHRateLimitUsage = float64(e.stats.TotalHHRequests) / float64(e.config.MaxAPIRequestsPerHour) * 100
	}
}

// scheduleJob - планирование задания
func (e *AutomationEngine) scheduleJob(job *AutomationJob) error {
	// Парсинг времени выполнения
	cronExpr := e.buildCronExpression(job.Schedule)

	entryID, err := e.cron.AddFunc(cronExpr, func() {
		e.executeScheduledJob(job)
	})

	if err != nil {
		return fmt.Errorf("failed to schedule cron job: %w", err)
	}

	// Сохранение ID задания
	e.runningJobs.Store(job.ID, entryID)

	e.logger.Info("Automation job scheduled",
		zap.String("job_id", job.ID.String()),
		zap.String("cron", cronExpr),
		zap.String("user_id", job.UserID.String()))

	return nil
}

// buildCronExpression - построение cron выражения
func (e *AutomationEngine) buildCronExpression(schedule AutomationSchedule) string {
	if schedule.Frequency == "daily" {
		// Разбор времени "HH:MM"
		parts := strings.Split(schedule.TimeOfDay, ":")
		if len(parts) != 2 {
			parts = []string{"8", "0"} // По умолчанию 08:00
		}

		hour := parts[0]
		minute := parts[1]

		return fmt.Sprintf("0 %s %s * * *", minute, hour)
	}

	// Для weekly - пока используем daily
	return "0 0 8 * * *"
}

// executeScheduledJob - выполнение запланированного задания
func (e *AutomationEngine) executeScheduledJob(job *AutomationJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	e.logger.Info("Executing scheduled automation job",
		zap.String("job_id", job.ID.String()),
		zap.String("user_id", job.UserID.String()))

	// Выполнение поиска и откликов
	result, err := e.performAutomatedSearch(ctx, job)
	if err != nil {
		e.logger.Error("Failed to execute scheduled job",
			zap.String("job_id", job.ID.String()),
			zap.String("user_id", job.UserID.String()),
			zap.Error(err))

		// Обновление статуса задания при ошибке
		job.LastError = err.Error()
		job.UpdatedAt = time.Now()

		if strings.Contains(err.Error(), "HH.ru") || strings.Contains(err.Error(), "token") {
			job.HHConnected = false
			job.Status = "hh_disconnected"
		}

		if updateErr := e.db.UpdateAutomationJob(ctx, job); updateErr != nil {
			e.logger.Error("Failed to update job after error",
				zap.String("job_id", job.ID.String()),
				zap.Error(updateErr))
		}
		return
	}

	// Обновление статистики
	job.Statistics.TotalRuns++
	job.Statistics.VacanciesFound += result.VacanciesFound
	job.Statistics.ApplicationsSent += result.ApplicationsSent
	job.Statistics.HHRequestsCount += result.HHRequestsCount

	now := time.Now()
	job.LastRun = &now
	nextRun := e.calculateNextRun(time.Now(), job.Schedule.TimeOfDay)
	job.NextRun = &nextRun
	job.LastError = ""
	job.HHConnected = true
	job.UpdatedAt = time.Now()

	// Сохранение обновлений
	if err := e.db.UpdateAutomationJob(ctx, job); err != nil {
		e.logger.Error("Failed to update job statistics",
			zap.String("job_id", job.ID.String()),
			zap.Error(err))
	}
}

// calculateNextRun - расчет времени следующего запуска
func (e *AutomationEngine) calculateNextRun(now time.Time, timeOfDay string) time.Time {
	// Разбор времени
	parts := strings.Split(timeOfDay, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])

	// Расчет следующего запуска (завтра в указанное время)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// StopAutomation - остановка автоматизации
func (e *AutomationEngine) StopAutomation(ctx context.Context, userID uuid.UUID) error {
	job, err := e.db.GetUserAutomationJob(ctx, userID)
	if err != nil {
		return fmt.Errorf("automation job not found: %w", err)
	}

	// Остановка cron job
	if entryID, ok := e.runningJobs.Load(job.ID); ok {
		e.cron.Remove(entryID.(cron.EntryID))
		e.runningJobs.Delete(job.ID)
	}

	// Обновление статуса
	job.Status = "paused"
	job.UpdatedAt = time.Now()

	if err := e.db.UpdateAutomationJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Отправка уведомления
	e.notifier.SendAutomationStopped(userID, job)

	e.logger.Info("Automation stopped",
		zap.String("user_id", userID.String()),
		zap.String("job_id", job.ID.String()))

	return nil
}

// GetAutomationStatus - получение статуса автоматизации
func (e *AutomationEngine) GetAutomationStatus(ctx context.Context, userID uuid.UUID) (*AutomationStatus, error) {
	job, err := e.db.GetUserAutomationJob(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("automation job not found: %w", err)
	}

	// Проверка подключения HH.ru
	hhConnected, _ := e.checkHHConnection(ctx, userID)
	job.HHConnected = hhConnected

	status := &AutomationStatus{
		JobID:       job.ID,
		UserID:      job.UserID,
		Status:      job.Status,
		Schedule:    job.Schedule,
		Stats:       job.Statistics,
		LastRun:     job.LastRun,
		NextRun:     job.NextRun,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		HHConnected: job.HHConnected,
		LastError:   job.LastError,
	}

	// Добавление сегодняшней статистики
	today := time.Now().Format("2006-01-02")
	applicationsToday, _ := e.db.GetUserApplicationsToday(ctx, userID, today)
	status.TodayStats = TodayStats{
		Applications: len(applicationsToday),
		LastSearch:   job.LastRun,
		HHRequests:   e.getHHRequestsToday(ctx, userID),
	}

	// Получение информации о rate limit
	allowed, waitTime, _ := e.hhService.CheckRateLimit(ctx, userID)
	status.RateLimit = RateLimitInfo{
		Allowed:  allowed,
		WaitTime: waitTime,
		Max:      e.config.MaxAPIRequestsPerHour,
		Used:     e.getHHRequestsThisHour(ctx, userID),
	}

	return status, nil
}

// getHHRequestsToday - получение количества HH.ru запросов сегодня
func (e *AutomationEngine) getHHRequestsToday(ctx context.Context, userID uuid.UUID) int {
	today := time.Now().Format("2006-01-02")
	total := 0

	// Суммируем запросы за каждый час сегодня
	for hour := 0; hour < 24; hour++ {
		key := fmt.Sprintf("hh_requests:user:%s:%s-%02d", userID.String(), today, hour)
		count, err := e.redis.GetInt(ctx, key)
		if err == nil {
			total += count
		}
	}

	return total
}

// getHHRequestsThisHour - получение количества HH.ru запросов в текущем часу
func (e *AutomationEngine) getHHRequestsThisHour(ctx context.Context, userID uuid.UUID) int {
	hour := time.Now().Format("2006-01-02-15")
	key := fmt.Sprintf("hh_requests:user:%s:%s", userID.String(), hour)

	count, err := e.redis.GetInt(ctx, key)
	if err != nil {
		return 0
	}

	return count
}

// ResumeAutomation - возобновление автоматизации после подключения HH.ru
func (e *AutomationEngine) ResumeAutomation(ctx context.Context, userID uuid.UUID) error {
	job, err := e.db.GetUserAutomationJob(ctx, userID)
	if err != nil {
		return fmt.Errorf("automation job not found: %w", err)
	}

	// Проверка подключения HH.ru
	hhConnected, err := e.checkHHConnection(ctx, userID)
	if err != nil || !hhConnected {
		return fmt.Errorf("HH.ru account not connected")
	}

	// Обновление статуса
	job.Status = "active"
	job.HHConnected = true
	job.UpdatedAt = time.Now()

	if err := e.db.UpdateAutomationJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Перезапуск планировщика
	if err := e.scheduleJob(job); err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	e.notifier.SendAutomationResumed(userID, job)

	e.logger.Info("Automation resumed after HH.ru reconnection",
		zap.String("user_id", userID.String()),
		zap.String("job_id", job.ID.String()))

	return nil
}

// Helper methods for mapping enums
func (e *AutomationEngine) mapExperience(exp string) string {
	switch exp {
	case "noExperience":
		return "noExperience"
	case "between1And3":
		return "between1And3"
	case "between3And6":
		return "between3And6"
	case "moreThan6":
		return "moreThan6"
	default:
		return "noExperience"
	}
}

func (e *AutomationEngine) mapEmployment(emp string) string {
	switch emp {
	case "full":
		return "full"
	case "part":
		return "part"
	case "project":
		return "project"
	case "volunteer":
		return "volunteer"
	case "probation":
		return "probation"
	default:
		return "full"
	}
}

func (e *AutomationEngine) mapSchedule(sched string) string {
	switch sched {
	case "fullDay":
		return "fullDay"
	case "shift":
		return "shift"
	case "flexible":
		return "flexible"
	case "remote":
		return "remote"
	case "flyInFlyOut":
		return "flyInFlyOut"
	default:
		return "fullDay"
	}
}

// Структуры данных
type AutomationResult struct {
	JobID            uuid.UUID `json:"job_id"`
	UserID           uuid.UUID `json:"user_id"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	VacanciesFound   int       `json:"vacancies_found"`
	NewVacancies     int       `json:"new_vacancies"`
	ApplicationsSent int       `json:"applications_sent"`
	AvgMatchScore    float64   `json:"avg_match_score"`
	HHRequestsCount  int       `json:"hh_requests_count"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
}

type AutomationReport struct {
	UserID           uuid.UUID     `json:"user_id"`
	JobID            uuid.UUID     `json:"job_id"`
	VacanciesFound   int           `json:"vacancies_found"`
	NewVacancies     int           `json:"new_vacancies"`
	ApplicationsSent int           `json:"applications_sent"`
	AvgMatchScore    float64       `json:"avg_match_score"`
	HHRequestsCount  int           `json:"hh_requests_count"`
	Duration         time.Duration `json:"duration"`
	Timestamp        time.Time     `json:"timestamp"`
}

type AutomationStatus struct {
	JobID       uuid.UUID          `json:"job_id"`
	UserID      uuid.UUID          `json:"user_id"`
	Status      string             `json:"status"`
	Schedule    AutomationSchedule `json:"schedule"`
	Stats       JobStatistics      `json:"stats"`
	TodayStats  TodayStats         `json:"today_stats"`
	RateLimit   RateLimitInfo      `json:"rate_limit"`
	LastRun     *time.Time         `json:"last_run,omitempty"`
	NextRun     *time.Time         `json:"next_run,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	HHConnected bool               `json:"hh_connected"`
	LastError   string             `json:"last_error,omitempty"`
}

type TodayStats struct {
	Applications int        `json:"applications"`
	LastSearch   *time.Time `json:"last_search,omitempty"`
	HHRequests   int        `json:"hh_requests"`
}

type RateLimitInfo struct {
	Allowed  bool          `json:"allowed"`
	WaitTime time.Duration `json:"wait_time,omitempty"`
	Max      int           `json:"max"`
	Used     int           `json:"used"`
}

// SmartMatcher - обновленный матчинг с учетом HH.ru данных
type SmartMatcher struct {
	logger *zap.Logger
}

func NewSmartMatcher(logger *zap.Logger) *SmartMatcher {
	return &SmartMatcher{logger: logger}
}

func (m *SmartMatcher) MatchVacancy(ctx context.Context, vacancy models.HHVacancy, resume models.HHResume) (*MatchResult, error) {
	score := 0.0

	// 1. Сравнение навыков (40%)
	skillScore := m.matchSkills(vacancy.KeySkills, resume.Skills)
	score += skillScore * 0.4

	// 2. Сравнение зарплаты (30%)
	salaryScore := m.matchSalary(vacancy.Salary, resume.Salary)
	score += salaryScore * 0.3

	// 3. Сравнение опыта (20%)
	experienceScore := m.matchExperience(vacancy.Experience, resume.Experience)
	score += experienceScore * 0.2

	// 4. Сравнение местоположения (10%)
	locationScore := m.matchLocation(vacancy.Area.Name, resume.Location)
	score += locationScore * 0.1

	// Извлечение совпадающих навыков
	matchedSkills := m.getMatchedSkills(vacancy.KeySkills, resume.Skills)

	return &MatchResult{
		Score:         score,
		MatchedSkills: matchedSkills,
		SkillScore:    skillScore,
		SalaryScore:   salaryScore,
		ExpScore:      experienceScore,
		LocationScore: locationScore,
	}, nil
}

func (m *SmartMatcher) matchSkills(vacancySkills []string, resumeSkills []string) float64 {
	if len(vacancySkills) == 0 {
		return 1.0
	}

	matched := 0
	vacancySkillsLower := make([]string, len(vacancySkills))
	for i, skill := range vacancySkills {
		vacancySkillsLower[i] = strings.ToLower(skill)
	}

	for _, vSkill := range vacancySkillsLower {
		for _, rSkill := range resumeSkills {
			if strings.Contains(strings.ToLower(rSkill), vSkill) {
				matched++
				break
			}
		}
	}

	return float64(matched) / float64(len(vacancySkills))
}

func (m *SmartMatcher) matchSalary(vacancySalary *models.Salary, resumeSalary *models.Salary) float64 {
	if vacancySalary == nil || resumeSalary == nil {
		return 0.5
	}

	vacancyAvg := vacancySalary.From
	if vacancySalary.To > 0 {
		vacancyAvg = (vacancySalary.From + vacancySalary.To) / 2
	}

	resumeAvg := resumeSalary.From
	if resumeSalary.To > 0 {
		resumeAvg = (resumeSalary.From + resumeSalary.To) / 2
	}

	// Разница в процентах
	if vacancyAvg == 0 {
		return 0.5
	}

	diff := math.Abs(float64(vacancyAvg-resumeAvg)) / float64(vacancyAvg)

	// Score от 1.0 (идеально) до 0.0 (большая разница)
	if diff <= 0.1 { // ±10%
		return 1.0
	} else if diff <= 0.2 { // ±20%
		return 0.8
	} else if diff <= 0.3 { // ±30%
		return 0.5
	}

	return 0.2
}

func (m *SmartMatcher) matchExperience(vacancyExp models.Experience, resumeExp []models.Experience) float64 {
	// Вакансия требует определенного опыта
	var requiredYears int
	switch vacancyExp.Name {
	case "Нет опыта":
		requiredYears = 0
	case "От 1 года до 3 лет":
		requiredYears = 1
	case "От 3 до 6 лет":
		requiredYears = 3
	case "Более 6 лет":
		requiredYears = 6
	default:
		return 0.5
	}

	// Расчет общего опыта из резюме
	totalExperience := 0.0
	for _, exp := range resumeExp {
		start, err1 := time.Parse("2006-01-02", exp.StartDate)
		var end time.Time
		if exp.EndDate == "" || exp.EndDate == "present" {
			end = time.Now()
		} else {
			end, _ = time.Parse("2006-01-02", exp.EndDate)
		}

		if err1 == nil {
			years := end.Sub(start).Hours() / 24 / 365
			totalExperience += years
		}
	}

	if requiredYears == 0 {
		return 1.0
	}

	if totalExperience >= float64(requiredYears) {
		return 1.0
	}

	return totalExperience / float64(requiredYears)
}

func (m *SmartMatcher) matchLocation(vacancyLocation, resumeLocation string) float64 {
	if strings.Contains(strings.ToLower(vacancyLocation), "удален") ||
		strings.Contains(strings.ToLower(vacancyLocation), "remote") {
		return 1.0
	}

	if strings.Contains(strings.ToLower(vacancyLocation), strings.ToLower(resumeLocation)) {
		return 1.0
	}

	return 0.0
}

func (m *SmartMatcher) getMatchedSkills(vacancySkills, resumeSkills []string) []string {
	matched := []string{}

	for _, vSkill := range vacancySkills {
		vSkillLower := strings.ToLower(vSkill)
		for _, rSkill := range resumeSkills {
			if strings.Contains(strings.ToLower(rSkill), vSkillLower) {
				matched = append(matched, vSkill)
				break
			}
		}
	}

	return matched
}

type MatchResult struct {
	Score         float64  `json:"score"`
	MatchedSkills []string `json:"matched_skills"`
	SkillScore    float64  `json:"skill_score"`
	SalaryScore   float64  `json:"salary_score"`
	ExpScore      float64  `json:"exp_score"`
	LocationScore float64  `json:"location_score"`
}
