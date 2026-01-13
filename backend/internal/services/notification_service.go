package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/models"
	"autojobsearch/backend/internal/storage"
)

// NotificationType типы уведомлений
type NotificationType string

const (
	// Автоматизация
	NotificationAutomationStarted   NotificationType = "automation_started"
	NotificationAutomationStopped   NotificationType = "automation_stopped"
	NotificationAutomationResumed   NotificationType = "automation_resumed"
	NotificationAutomationCompleted NotificationType = "automation_completed"
	NotificationAutomationFailed    NotificationType = "automation_failed"

	// Отклики
	NotificationApplicationSent     NotificationType = "application_sent"
	NotificationApplicationFailed   NotificationType = "application_failed"
	NotificationApplicationViewed   NotificationType = "application_viewed"
	NotificationApplicationRejected NotificationType = "application_rejected"
	NotificationApplicationAccepted NotificationType = "application_accepted"

	// Приглашения
	NotificationInvitationReceived NotificationType = "invitation_received"
	NotificationInterviewScheduled NotificationType = "interview_scheduled"
	NotificationInterviewCancelled NotificationType = "interview_cancelled"

	// HH.ru
	NotificationHHConnected      NotificationType = "hh_connected"
	NotificationHHDisconnected   NotificationType = "hh_disconnected"
	NotificationHHConnectionLost NotificationType = "hh_connection_lost"
	NotificationHHTokensExpired  NotificationType = "hh_tokens_expired"

	// Системные
	NotificationDailyReport  NotificationType = "daily_report"
	NotificationWeeklyReport NotificationType = "weekly_report"
	NotificationSystemAlert  NotificationType = "system_alert"
)

// NotificationChannel каналы доставки уведомлений
type NotificationChannel string

const (
	ChannelInApp    NotificationChannel = "in_app"
	ChannelEmail    NotificationChannel = "email"
	ChannelPush     NotificationChannel = "push"
	ChannelSMS      NotificationChannel = "sms"
	ChannelTelegram NotificationChannel = "telegram"
)

// NotificationService сервис уведомлений
type NotificationService struct {
	db     *storage.Database
	redis  *storage.RedisClient
	logger *zap.Logger

	// Конфигурация
	emailEnabled    bool
	pushEnabled     bool
	smsEnabled      bool
	telegramEnabled bool
}

// NotificationConfig конфигурация уведомлений
type NotificationConfig struct {
	EmailEnabled    bool `json:"email_enabled"`
	PushEnabled     bool `json:"push_enabled"`
	SmsEnabled      bool `json:"sms_enabled"`
	TelegramEnabled bool `json:"telegram_enabled"`
}

// NewNotificationService создает новый сервис уведомлений
func NewNotificationService(
	db *storage.Database,
	redis *storage.RedisClient,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		db:     db,
		redis:  redis,
		logger: logger,

		// По умолчанию все каналы включены
		emailEnabled:    true,
		pushEnabled:     true,
		smsEnabled:      false,
		telegramEnabled: false,
	}
}

// NotificationRequest запрос на отправку уведомления
type NotificationRequest struct {
	UserID   uuid.UUID              `json:"user_id"`
	Type     NotificationType       `json:"type"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Channels []NotificationChannel  `json:"channels,omitempty"`
	Priority int                    `json:"priority,omitempty"` // 1-5, где 5 - самый высокий
}

// SendNotification отправка уведомления
func (s *NotificationService) SendNotification(ctx context.Context, req NotificationRequest) error {
	// Создание записи уведомления
	notification := &models.Notification{
		ID:        uuid.New(),
		UserID:    req.UserID,
		Type:      string(req.Type),
		Title:     req.Title,
		Message:   req.Message,
		Data:      req.Data,
		IsRead:    false,
		CreatedAt: time.Now(),
	}

	// Сохранение в БД
	if err := s.saveNotification(ctx, notification); err != nil {
		return fmt.Errorf("failed to save notification: %w", err)
	}

	// Определение каналов доставки
	channels := req.Channels
	if len(channels) == 0 {
		channels = s.getDefaultChannels(req.Type)
	}

	// Отправка через выбранные каналы
	for _, channel := range channels {
		switch channel {
		case ChannelInApp:
			// Уже сохранено в БД
			continue
		case ChannelEmail:
			s.sendEmailNotification(ctx, req)
		case ChannelPush:
			s.sendPushNotification(ctx, req)
		case ChannelSMS:
			s.sendSMSNotification(ctx, req)
		case ChannelTelegram:
			s.sendTelegramNotification(ctx, req)
		}
	}

	// Логирование
	s.logger.Info("Notification sent",
		zap.String("type", string(req.Type)),
		zap.String("user_id", req.UserID.String()),
		zap.Any("channels", channels))

	return nil
}

// getDefaultChannels возвращает каналы по умолчанию для типа уведомления
func (s *NotificationService) getDefaultChannels(notificationType NotificationType) []NotificationChannel {
	channels := []NotificationChannel{ChannelInApp}

	// Критичные уведомления отправляем везде
	switch notificationType {
	case NotificationSystemAlert,
		NotificationHHConnectionLost,
		NotificationHHTokensExpired:

		if s.emailEnabled {
			channels = append(channels, ChannelEmail)
		}
		if s.pushEnabled {
			channels = append(channels, ChannelPush)
		}

	// Важные уведомления - email + push
	case NotificationInvitationReceived,
		NotificationInterviewScheduled,
		NotificationApplicationAccepted:

		if s.emailEnabled {
			channels = append(channels, ChannelEmail)
		}
		if s.pushEnabled {
			channels = append(channels, ChannelPush)
		}

	// Обычные уведомления - только in-app
	default:
		// Только in-app
	}

	return channels
}

// saveNotification сохранение уведомления в БД
func (s *NotificationService) saveNotification(ctx context.Context, notification *models.Notification) error {
	query := `
        INSERT INTO notifications (id, user_id, type, title, message, data, is_read, created_at)
        VALUES (:id, :user_id, :type, :title, :message, :data, :is_read, :created_at)
    `

	// Сериализация data
	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}

	notification.Data = map[string]interface{}{
		"json": string(dataJSON),
	}

	_, err = s.db.ExecContext(ctx, query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Message,
		notification.Data,
		notification.IsRead,
		notification.CreatedAt,
	)

	return err
}

// Методы отправки через разные каналы

func (s *NotificationService) sendEmailNotification(ctx context.Context, req NotificationRequest) {
	// Реализация отправки email
	s.logger.Debug("Email notification prepared",
		zap.String("user_id", req.UserID.String()),
		zap.String("type", string(req.Type)))

	// В MVP просто логируем
	// В продакшене здесь будет интеграция с SMTP сервером
}

func (s *NotificationService) sendPushNotification(ctx context.Context, req NotificationRequest) {
	// Получение device tokens пользователя
	deviceTokens := s.getUserDeviceTokens(ctx, req.UserID)

	if len(deviceTokens) == 0 {
		return
	}

	// Подготовка push уведомления
	pushData := map[string]interface{}{
		"notification_id": uuid.New().String(),
		"type":            req.Type,
		"title":           req.Title,
		"body":            req.Message,
		"data":            req.Data,
		"priority":        req.Priority,
		"timestamp":       time.Now().Unix(),
	}

	// Отправка через Firebase/APNS
	for _, token := range deviceTokens {
		s.sendToPushService(ctx, token, pushData)
	}
}

func (s *NotificationService) sendSMSNotification(ctx context.Context, req NotificationRequest) {
	// Реализация отправки SMS
	s.logger.Debug("SMS notification prepared",
		zap.String("user_id", req.UserID.String()))
}

func (s *NotificationService) sendTelegramNotification(ctx context.Context, req NotificationRequest) {
	// Реализация отправки в Telegram
	s.logger.Debug("Telegram notification prepared",
		zap.String("user_id", req.UserID.String()))
}

// Вспомогательные методы

func (s *NotificationService) getUserDeviceTokens(ctx context.Context, userID uuid.UUID) []string {
	// Получение device tokens из Redis или БД
	key := fmt.Sprintf("user:%s:device_tokens", userID.String())

	tokensJSON, err := s.redis.Get(ctx, key)
	if err != nil {
		return []string{}
	}

	var tokens []string
	if err := json.Unmarshal([]byte(tokensJSON), &tokens); err != nil {
		return []string{}
	}

	return tokens
}

func (s *NotificationService) sendToPushService(ctx context.Context, deviceToken string, data map[string]interface{}) {
	// Интеграция с Firebase Cloud Messaging (FCM) или APNS
	// Для MVP просто логируем

	s.logger.Debug("Push notification to device",
		zap.String("device_token", maskToken(deviceToken)),
		zap.Any("data", data))
}

// Методы для конкретных типов уведомлений

// SendAutomationStarted уведомление о запуске автоматизации
func (s *NotificationService) SendAutomationStarted(userID uuid.UUID, job *AutomationJob) {
	req := NotificationRequest{
		UserID:  userID,
		Type:    NotificationAutomationStarted,
		Title:   "Автоматизация запущена",
		Message: fmt.Sprintf("Автоматический поиск вакансий запущен. Следующий запуск: %s", job.NextRun.Format("02.01.2006 15:04")),
		Data: map[string]interface{}{
			"job_id":     job.ID.String(),
			"next_run":   job.NextRun,
			"settings":   job.Settings,
			"created_at": job.CreatedAt,
		},
		Priority: 3,
	}

	go s.SendNotification(context.Background(), req)
}

// SendAutomationStopped уведомление об остановке автоматизации
func (s *NotificationService) SendAutomationStopped(userID uuid.UUID, job *AutomationJob) {
	req := NotificationRequest{
		UserID:  userID,
		Type:    NotificationAutomationStopped,
		Title:   "Автоматизация остановлена",
		Message: "Автоматический поиск вакансий остановлен. Вы можете возобновить его в любое время.",
		Data: map[string]interface{}{
			"job_id":       job.ID.String(),
			"total_runs":   job.Statistics.TotalRuns,
			"applications": job.Statistics.ApplicationsSent,
			"last_run":     job.LastRun,
		},
		Priority: 3,
	}

	go s.SendNotification(context.Background(), req)
}

// SendAutomationReport отчет о выполненной автоматизации
func (s *NotificationService) SendAutomationReport(userID uuid.UUID, report *AutomationReport) {
	req := NotificationRequest{
		UserID: userID,
		Type:   NotificationAutomationCompleted,
		Title:  "Отчет об автоматическом поиске",
		Message: fmt.Sprintf("Найдено вакансий: %d, отправлено откликов: %d",
			report.VacanciesFound, report.ApplicationsSent),
		Data: map[string]interface{}{
			"report_id":         uuid.New().String(),
			"vacancies_found":   report.VacanciesFound,
			"applications_sent": report.ApplicationsSent,
			"duration":          report.Duration.Seconds(),
			"avg_match_score":   report.AvgMatchScore,
			"timestamp":         report.Timestamp,
		},
		Priority: 2,
	}

	go s.SendNotification(context.Background(), req)
}

// SendApplicationSent уведомление об отправленном отклике
func (s *NotificationService) SendApplicationSent(userID uuid.UUID, application *models.Application) {
	req := NotificationRequest{
		UserID:  userID,
		Type:    NotificationApplicationSent,
		Title:   fmt.Sprintf("Отклик отправлен в %s", application.CompanyName),
		Message: fmt.Sprintf("Ваш отклик на вакансию \"%s\" успешно отправлен", application.VacancyTitle),
		Data: map[string]interface{}{
			"application_id": application.ID.String(),
			"vacancy_title":  application.VacancyTitle,
			"company_name":   application.CompanyName,
			"match_score":    application.MatchScore,
			"applied_at":     application.AppliedAt,
			"automated":      application.Automated,
		},
		Priority: 2,
	}

	go s.SendNotification(context.Background(), req)
}

// SendInvitationReceived уведомление о получении приглашения
func (s *NotificationService) SendInvitationReceived(userID uuid.UUID, invitation *models.Invitation) {
	req := NotificationRequest{
		UserID: userID,
		Type:   NotificationInvitationReceived,
		Title:  fmt.Sprintf("Приглашение от %s", invitation.CompanyName),
		Message: fmt.Sprintf("Компания %s приглашает вас на собеседование на позицию %s",
			invitation.CompanyName, invitation.Position),
		Data: map[string]interface{}{
			"invitation_id":  invitation.ID.String(),
			"company_name":   invitation.CompanyName,
			"position":       invitation.Position,
			"interview_date": invitation.InterviewDate,
			"received_at":    invitation.ReceivedAt,
		},
		Priority: 5, // Высший приоритет
		Channels: []NotificationChannel{
			ChannelInApp,
			ChannelEmail,
			ChannelPush,
		},
	}

	go s.SendNotification(context.Background(), req)
}

// SendHHConnectionLost уведомление о потере соединения с HH.ru
func (s *NotificationService) SendHHConnectionLost(userID uuid.UUID) {
	req := NotificationRequest{
		UserID:  userID,
		Type:    NotificationHHConnectionLost,
		Title:   "Потеряно соединение с HH.ru",
		Message: "Не удалось подключиться к HH.ru. Автоматизация приостановлена. Пожалуйста, проверьте подключение.",
		Data: map[string]interface{}{
			"user_id": userID.String(),
			"time":    time.Now(),
			"action":  "reconnect_required",
		},
		Priority: 4,
		Channels: []NotificationChannel{
			ChannelInApp,
			ChannelEmail,
		},
	}

	go s.SendNotification(context.Background(), req)
}

// SendDailyReport ежедневный отчет
func (s *NotificationService) SendDailyReport(userID uuid.UUID, stats map[string]interface{}) {
	applications := stats["applications_today"].(int)
	invitations := stats["invitations_today"].(int)

	req := NotificationRequest{
		UserID: userID,
		Type:   NotificationDailyReport,
		Title:  "Ежедневный отчет",
		Message: fmt.Sprintf("За сегодня отправлено %d откликов, получено %d приглашений",
			applications, invitations),
		Data:     stats,
		Priority: 1,
	}

	go s.SendNotification(context.Background(), req)
}

// RegisterDeviceToken регистрация device token для push уведомлений
func (s *NotificationService) RegisterDeviceToken(ctx context.Context, userID uuid.UUID, deviceToken, platform string) error {
	key := fmt.Sprintf("user:%s:device_tokens", userID.String())

	// Получение текущих токенов
	tokens := s.getUserDeviceTokens(ctx, userID)

	// Добавление нового токена
	tokens = append(tokens, deviceToken)

	// Удаление дубликатов
	tokens = removeDuplicates(tokens)

	// Сохранение в Redis
	tokensJSON, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	if err := s.redis.SetWithExpiry(ctx, key, string(tokensJSON), 30*24*time.Hour); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	// Сохранение информации об устройстве
	deviceKey := fmt.Sprintf("device:%s", deviceToken)
	deviceInfo := map[string]interface{}{
		"user_id":     userID.String(),
		"platform":    platform,
		"registered":  time.Now(),
		"last_active": time.Now(),
	}

	deviceJSON, _ := json.Marshal(deviceInfo)
	s.redis.SetWithExpiry(ctx, deviceKey, string(deviceJSON), 30*24*time.Hour)

	s.logger.Info("Device token registered",
		zap.String("user_id", userID.String()),
		zap.String("platform", platform))

	return nil
}

// UnregisterDeviceToken удаление device token
func (s *NotificationService) UnregisterDeviceToken(ctx context.Context, deviceToken string) error {
	// Получение информации об устройстве
	deviceKey := fmt.Sprintf("device:%s", deviceToken)
	deviceInfoJSON, err := s.redis.Get(ctx, deviceKey)
	if err != nil {
		return nil // Устройство не найдено
	}

	var deviceInfo map[string]interface{}
	if err := json.Unmarshal([]byte(deviceInfoJSON), &deviceInfo); err != nil {
		return fmt.Errorf("failed to parse device info: %w", err)
	}

	userID, _ := uuid.Parse(deviceInfo["user_id"].(string))

	// Удаление из списка токенов пользователя
	key := fmt.Sprintf("user:%s:device_tokens", userID.String())
	tokens := s.getUserDeviceTokens(ctx, userID)

	// Фильтрация токенов
	newTokens := []string{}
	for _, token := range tokens {
		if token != deviceToken {
			newTokens = append(newTokens, token)
		}
	}

	// Сохранение обновленного списка
	tokensJSON, err := json.Marshal(newTokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	s.redis.SetWithExpiry(ctx, key, string(tokensJSON), 30*24*time.Hour)

	// Удаление информации об устройстве
	s.redis.Delete(ctx, deviceKey)

	s.logger.Info("Device token unregistered",
		zap.String("user_id", userID.String()),
		zap.String("device_token", maskToken(deviceToken)))

	return nil
}

// Вспомогательные функции

func removeDuplicates(tokens []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, token := range tokens {
		if !seen[token] {
			seen[token] = true
			result = append(result, token)
		}
	}

	return result
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
