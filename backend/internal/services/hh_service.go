package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"autojobsearch/internal/models"
	"autojobsearch/internal/storage"
)

// HHServiceConfig конфигурация HH.ru OAuth
type HHServiceConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	APIBaseURL   string `json:"api_base_url"`
}

// UserHHTokens OAuth токены пользователя для HH.ru
type UserHHTokens struct {
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	AccessToken  string    `json:"access_token" db:"access_token" encrypt:"true"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token" encrypt:"true"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	TokenType    string    `json:"token_type" db:"token_type"`
	Scope        string    `json:"scope" db:"scope"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	// Кэш для быстрого доступа
	isExpired bool
	mu        sync.RWMutex
}

// HHService сервис для работы с HH.ru API от имени пользователя
type HHService struct {
	config      *HHServiceConfig
	oauthConfig *oauth2.Config
	db          *storage.Database
	redis       *storage.RedisClient
	logger      *zap.Logger
	httpClient  *http.Client
	tokenCache  sync.Map // userID -> *UserHHTokens
}

// NewHHService создает новый сервис HH.ru
func NewHHService(config *HHServiceConfig, db *storage.Database, redis *storage.RedisClient, logger *zap.Logger) *HHService {
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Scopes:       []string{"read_applications", "write_applications", "read_resumes", "vacancies"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.AuthURL,
			TokenURL: config.TokenURL,
		},
	}

	return &HHService{
		config:      config,
		oauthConfig: oauthConfig,
		db:          db,
		redis:       redis,
		logger:      logger,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAuthorizationURL возвращает URL для авторизации пользователя в HH.ru
func (s *HHService) GetAuthorizationURL(userID uuid.UUID, state string) string {
	return s.oauthConfig.AuthCodeURL(state, oauth2.SetAuthURLParam("user_id", userID.String()))
}

// ExchangeCode обменяет код авторизации на токены пользователя
func (s *HHService) ExchangeCode(ctx context.Context, userID uuid.UUID, code string) (*UserHHTokens, error) {
	s.logger.Info("Exchanging authorization code for tokens",
		zap.String("user_id", userID.String()))

	// Получение токенов от HH.ru
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		s.logger.Error("Failed to exchange code for tokens",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Сохранение токенов пользователя
	userTokens := &UserHHTokens{
		UserID:       userID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
		TokenType:    token.TokenType,
		Scope:        strings.Join(token.Extra("scope").([]string), " "),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Сохранение в БД
	if err := s.db.SaveHHTokens(ctx, userTokens); err != nil {
		return nil, fmt.Errorf("failed to save tokens: %w", err)
	}

	// Кэширование в памяти
	s.tokenCache.Store(userID, userTokens)

	s.logger.Info("Successfully exchanged code for tokens",
		zap.String("user_id", userID.String()),
		zap.Time("expires_at", userTokens.ExpiresAt))

	return userTokens, nil
}

// GetOrRefreshTokens получает или обновляет токены пользователя
func (s *HHService) GetOrRefreshTokens(ctx context.Context, userID uuid.UUID) (*UserHHTokens, error) {
	// Проверка кэша в памяти
	if cached, ok := s.tokenCache.Load(userID); ok {
		tokens := cached.(*UserHHTokens)
		if !tokens.IsExpired() {
			return tokens, nil
		}
	}

	// Получение из БД
	tokens, err := s.db.GetHHTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tokens from DB: %w", err)
	}

	// Если токены просрочены, обновляем их
	if tokens.IsExpired() {
		s.logger.Info("Tokens expired, refreshing",
			zap.String("user_id", userID.String()))

		refreshed, err := s.refreshTokens(ctx, tokens)
		if err != nil {
			// Если refresh не удался, удаляем токены
			s.logger.Error("Failed to refresh tokens, removing from DB",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			s.db.DeleteHHTokens(ctx, userID)
			return nil, fmt.Errorf("tokens expired and refresh failed: %w", err)
		}
		tokens = refreshed
	}

	// Обновление кэша
	s.tokenCache.Store(userID, tokens)

	return tokens, nil
}

// refreshTokens обновляет access token с помощью refresh token
func (s *HHService) refreshTokens(ctx context.Context, tokens *UserHHTokens) (*UserHHTokens, error) {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()

	// Создание запроса на обновление токена
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", tokens.RefreshToken)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s", tokens.UserID.String()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed with status: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	// Обновление токенов
	tokens.AccessToken = result.AccessToken
	tokens.RefreshToken = result.RefreshToken
	tokens.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	tokens.TokenType = result.TokenType
	tokens.Scope = result.Scope
	tokens.UpdatedAt = time.Now()

	// Сохранение в БД
	if err := s.db.UpdateHHTokens(ctx, tokens); err != nil {
		return nil, fmt.Errorf("failed to save refreshed tokens: %w", err)
	}

	s.logger.Info("Tokens refreshed successfully",
		zap.String("user_id", tokens.UserID.String()),
		zap.Time("new_expires_at", tokens.ExpiresAt))

	return tokens, nil
}

// IsExpired проверяет, истек ли срок действия токенов
func (t *UserHHTokens) IsExpired() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.isExpired {
		return true
	}

	// Добавляем буфер в 5 минут для обновления до истечения срока
	isExpired := time.Until(t.ExpiresAt) < 5*time.Minute
	t.isExpired = isExpired

	return isExpired
}

// SearchVacancies поиск вакансий от имени конкретного пользователя
func (s *HHService) SearchVacancies(ctx context.Context, userID uuid.UUID, params map[string]string) ([]models.HHVacancy, error) {
	// Получение токенов пользователя
	tokens, err := s.GetOrRefreshTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}

	// Формирование URL запроса
	apiURL := s.config.APIBaseURL + "/vacancies"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавление параметров запроса
	q := req.URL.Query()
	for key, value := range params {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	// Установка заголовков с user-specific токеном
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s/1.0", userID.String()))
	req.Header.Set("HH-User-Agent", fmt.Sprintf("AutoJobSearch/1.0 (user_id: %s)", userID.String()))

	// Выполнение запроса
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search vacancies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HH.ru API error: %d", resp.StatusCode)
	}

	// Парсинг ответа
	var result struct {
		Items   []models.HHVacancy `json:"items"`
		Found   int                `json:"found"`
		Pages   int                `json:"pages"`
		Page    int                `json:"page"`
		PerPage int                `json:"per_page"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode vacancies response: %w", err)
	}

	// Логирование для аудита
	s.logAuditEvent(ctx, userID, "search_vacancies", params, len(result.Items))

	return result.Items, nil
}

// GetVacancy получение конкретной вакансии от имени пользователя
func (s *HHService) GetVacancy(ctx context.Context, userID uuid.UUID, vacancyID string) (*models.HHVacancy, error) {
	tokens, err := s.GetOrRefreshTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}

	apiURL := fmt.Sprintf("%s/vacancies/%s", s.config.APIBaseURL, vacancyID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s/1.0", userID.String()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get vacancy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HH.ru API error: %d", resp.StatusCode)
	}

	var vacancy models.HHVacancy
	if err := json.NewDecoder(resp.Body).Decode(&vacancy); err != nil {
		return nil, fmt.Errorf("failed to decode vacancy response: %w", err)
	}

	s.logAuditEvent(ctx, userID, "get_vacancy", map[string]string{"vacancy_id": vacancyID}, 1)

	return &vacancy, nil
}

// SendApplication отправка отклика на вакансию от имени пользователя
func (s *HHService) SendApplication(ctx context.Context, userID uuid.UUID, vacancyID string, application *models.Application) error {
	tokens, err := s.GetOrRefreshTokens(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	// Получение резюме пользователя из HH.ru
	resumes, err := s.getUserResumes(ctx, userID, tokens)
	if err != nil {
		return fmt.Errorf("failed to get user resumes: %w", err)
	}

	if len(resumes) == 0 {
		return fmt.Errorf("user has no resumes on HH.ru")
	}

	// Используем первое доступное резюме
	resumeID := resumes[0].ID

	// Отправка отклика через HH.ru API
	apiURL := fmt.Sprintf("%s/negotiations", s.config.APIBaseURL)

	requestBody := map[string]interface{}{
		"vacancy_id": vacancyID,
		"resume_id":  resumeID,
		"message":    application.CoverLetter,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal application: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("failed to create application request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s/1.0", userID.String()))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send application: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Description string `json:"description"`
			Errors      []struct {
				Value string `json:"value"`
				Type  string `json:"type"`
			} `json:"errors"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			return fmt.Errorf("HH.ru application error: %s", errorResp.Description)
		}

		return fmt.Errorf("HH.ru API error: %d", resp.StatusCode)
	}

	// Сохранение ID отклика из HH.ru
	var result struct {
		Id string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Id != "" {
		application.HHApplicationID = result.Id
	}

	s.logAuditEvent(ctx, userID, "send_application", map[string]string{
		"vacancy_id": vacancyID,
		"resume_id":  resumeID,
	}, 1)

	s.logger.Info("Application sent successfully via HH.ru",
		zap.String("user_id", userID.String()),
		zap.String("vacancy_id", vacancyID),
		zap.String("hh_application_id", application.HHApplicationID))

	return nil
}

// getUserResumes получает резюме пользователя с HH.ru
func (s *HHService) getUserResumes(ctx context.Context, userID uuid.UUID, tokens *UserHHTokens) ([]models.HHResume, error) {
	apiURL := s.config.APIBaseURL + "/resumes/mine"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resumes request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s/1.0", userID.String()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get resumes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HH.ru API error for resumes: %d", resp.StatusCode)
	}

	var resumes []models.HHResume
	if err := json.NewDecoder(resp.Body).Decode(&resumes); err != nil {
		return nil, fmt.Errorf("failed to decode resumes response: %w", err)
	}

	return resumes, nil
}

// logAuditEvent логирование действий пользователя для аудита
func (s *HHService) logAuditEvent(ctx context.Context, userID uuid.UUID, action string, params map[string]string, resultCount int) {
	auditLog := map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"user_id":      userID.String(),
		"action":       action,
		"params":       params,
		"result_count": resultCount,
		"user_agent":   fmt.Sprintf("AutoJobSearch/User/%s", userID.String()),
	}

	// Сохранение в Redis для быстрого доступа
	auditKey := fmt.Sprintf("audit:user:%s:%s:%d",
		userID.String(),
		action,
		time.Now().Unix())

	auditJSON, _ := json.Marshal(auditLog)
	s.redis.SetWithExpiry(ctx, auditKey, string(auditJSON), 24*time.Hour)

	// Также логируем в основной логгер
	s.logger.Info("HH.ru API audit",
		zap.String("user_id", userID.String()),
		zap.String("action", action),
		zap.Any("params", params),
		zap.Int("result_count", resultCount))
}

// GetUserInfo получение информации о пользователе с HH.ru
func (s *HHService) GetUserInfo(ctx context.Context, userID uuid.UUID) (*models.HHUserInfo, error) {
	tokens, err := s.GetOrRefreshTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}

	apiURL := s.config.APIBaseURL + "/me"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("AutoJobSearch/User/%s/1.0", userID.String()))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HH.ru API error for user info: %d", resp.StatusCode)
	}

	var userInfo models.HHUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info response: %w", err)
	}

	return &userInfo, nil
}

// CheckRateLimit проверка лимитов API для конкретного пользователя
func (s *HHService) CheckRateLimit(ctx context.Context, userID uuid.UUID) (bool, time.Duration, error) {
	key := fmt.Sprintf("rate_limit:hh:user:%s", userID.String())

	// Получаем текущий счетчик из Redis
	count, err := s.redis.GetInt(ctx, key)
	if err != nil {
		// Если ключа нет, начинаем новый интервал
		count = 0
	}

	// HH.ru лимиты: обычно 500 запросов в час на пользователя
	maxRequests := 500
	window := time.Hour

	if count >= maxRequests {
		// Получаем TTL ключа для расчета времени до сброса
		ttl, err := s.redis.TTL(ctx, key)
		if err != nil {
			return false, 0, err
		}
		return false, ttl, nil
	}

	// Увеличиваем счетчик
	if count == 0 {
		// Первый запрос в интервале, устанавливаем TTL
		s.redis.SetWithExpiry(ctx, key, "1", window)
	} else {
		s.redis.Increment(ctx, key)
	}

	return true, 0, nil
}
