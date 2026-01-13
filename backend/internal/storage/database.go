package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/models"
)

// Database обертка над sqlx.DB
type Database struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewDatabase создает новое подключение к БД
func NewDatabase(dsn string, logger *zap.Logger) (*Database, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Проверка соединения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	logger.Info("Database connection established")

	return &Database{
		db:     db,
		logger: logger,
	}, nil
}

// Close закрывает подключение к БД
func (d *Database) Close() error {
	return d.db.Close()
}

// User operations

// CreateUser создает нового пользователя
func (d *Database) CreateUser(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (id, email, password, first_name, last_name, is_active, created_at, updated_at)
        VALUES (:id, :email, :password, :first_name, :last_name, :is_active, :created_at, :updated_at)
    `

	_, err := d.db.NamedExecContext(ctx, query, user)
	return err
}

// GetUserByID получает пользователя по ID
func (d *Database) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = $1`

	err := d.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByEmail получает пользователя по email
func (d *Database) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE email = $1`

	err := d.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// UpdateUser обновляет пользователя
func (d *Database) UpdateUser(ctx context.Context, user *models.User) error {
	query := `
        UPDATE users 
        SET email = :email, first_name = :first_name, last_name = :last_name, 
            phone = :phone, avatar_url = :avatar_url, updated_at = :updated_at
        WHERE id = :id
    `

	_, err := d.db.NamedExecContext(ctx, query, user)
	return err
}

// Automation operations

// SaveAutomationJob сохраняет задание автоматизации
func (d *Database) SaveAutomationJob(ctx context.Context, job *models.AutomationJob) error {
	query := `
        INSERT INTO automation_jobs (id, user_id, schedule, settings, status, statistics, 
                                     last_run, next_run, hh_connected, last_error, created_at, updated_at)
        VALUES (:id, :user_id, :schedule, :settings, :status, :statistics, 
                :last_run, :next_run, :hh_connected, :last_error, :created_at, :updated_at)
        ON CONFLICT (id) DO UPDATE SET
            schedule = EXCLUDED.schedule,
            settings = EXCLUDED.settings,
            status = EXCLUDED.status,
            statistics = EXCLUDED.statistics,
            last_run = EXCLUDED.last_run,
            next_run = EXCLUDED.next_run,
            hh_connected = EXCLUDED.hh_connected,
            last_error = EXCLUDED.last_error,
            updated_at = EXCLUDED.updated_at
    `

	_, err := d.db.NamedExecContext(ctx, query, job)
	return err
}

// GetUserAutomationJob получает задание автоматизации пользователя
func (d *Database) GetUserAutomationJob(ctx context.Context, userID uuid.UUID) (*models.AutomationJob, error) {
	var job models.AutomationJob
	query := `SELECT * FROM automation_jobs WHERE user_id = $1`

	err := d.db.GetContext(ctx, &job, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &job, nil
}

// UpdateAutomationJob обновляет задание автоматизации
func (d *Database) UpdateAutomationJob(ctx context.Context, job *models.AutomationJob) error {
	query := `
        UPDATE automation_jobs 
        SET schedule = :schedule, settings = :settings, status = :status, 
            statistics = :statistics, last_run = :last_run, next_run = :next_run,
            hh_connected = :hh_connected, last_error = :last_error, updated_at = :updated_at
        WHERE id = :id
    `

	_, err := d.db.NamedExecContext(ctx, query, job)
	return err
}

// DeleteAutomationJob удаляет задание автоматизации
func (d *Database) DeleteAutomationJob(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM automation_jobs WHERE id = $1`
	_, err := d.db.ExecContext(ctx, query, id)
	return err
}

// Resume operations

// SaveResume сохраняет резюме
func (d *Database) SaveResume(ctx context.Context, resume *models.Resume) error {
	query := `
        INSERT INTO resumes (id, user_id, title, file_path, file_type, file_size, 
                            parsed_data, is_primary, hh_resume_id, created_at, updated_at)
        VALUES (:id, :user_id, :title, :file_path, :file_type, :file_size, 
                :parsed_data, :is_primary, :hh_resume_id, :created_at, :updated_at)
    `

	_, err := d.db.NamedExecContext(ctx, query, resume)
	return err
}

// GetUserResumes получает резюме пользователя
func (d *Database) GetUserResumes(ctx context.Context, userID uuid.UUID) ([]models.Resume, error) {
	var resumes []models.Resume
	query := `SELECT * FROM resumes WHERE user_id = $1 ORDER BY is_primary DESC, created_at DESC`

	err := d.db.SelectContext(ctx, &resumes, query, userID)
	if err != nil {
		return nil, err
	}

	return resumes, nil
}

// GetPrimaryResume получает основное резюме пользователя
func (d *Database) GetPrimaryResume(ctx context.Context, userID uuid.UUID) (*models.Resume, error) {
	var resume models.Resume
	query := `SELECT * FROM resumes WHERE user_id = $1 AND is_primary = true LIMIT 1`

	err := d.db.GetContext(ctx, &resume, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &resume, nil
}

// UpdateResume обновляет резюме
func (d *Database) UpdateResume(ctx context.Context, resume *models.Resume) error {
	query := `
        UPDATE resumes 
        SET title = :title, parsed_data = :parsed_data, is_primary = :is_primary,
            hh_resume_id = :hh_resume_id, updated_at = :updated_at
        WHERE id = :id
    `

	_, err := d.db.NamedExecContext(ctx, query, resume)
	return err
}

// DeleteResume удаляет резюме
func (d *Database) DeleteResume(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM resumes WHERE id = $1`
	_, err := d.db.ExecContext(ctx, query, id)
	return err
}

// Application operations

// SaveApplication сохраняет отклик
func (d *Database) SaveApplication(ctx context.Context, app *models.Application) error {
	query := `
        INSERT INTO applications (id, user_id, vacancy_id, vacancy_title, company_name, 
                                 resume_id, cover_letter, status, match_score, applied_at, 
                                 automated, source, hh_application_id, error_message, 
                                 vacancy_url, created_at, updated_at)
        VALUES (:id, :user_id, :vacancy_id, :vacancy_title, :company_name, 
                :resume_id, :cover_letter, :status, :match_score, :applied_at, 
                :automated, :source, :hh_application_id, :error_message, 
                :vacancy_url, :created_at, :updated_at)
    `

	_, err := d.db.NamedExecContext(ctx, query, app)
	return err
}

// GetUserApplications получает отклики пользователя
func (d *Database) GetUserApplications(ctx context.Context, userID uuid.UUID, page, limit int, status string) ([]models.Application, int, error) {
	var apps []models.Application

	baseQuery := `SELECT * FROM applications WHERE user_id = $1`
	countQuery := `SELECT COUNT(*) FROM applications WHERE user_id = $1`

	args := []interface{}{userID}
	argIndex := 2

	if status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	baseQuery += " ORDER BY applied_at DESC"

	// Пагинация
	offset := (page - 1) * limit
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	// Получение данных
	err := d.db.SelectContext(ctx, &apps, baseQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// Подсчет общего количества
	var total int
	countArgs := args[:len(args)-2] // Убираем LIMIT и OFFSET
	err = d.db.GetContext(ctx, &total, countQuery, countArgs...)
	if err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// GetUserApplicationsToday получает отклики пользователя за сегодня
func (d *Database) GetUserApplicationsToday(ctx context.Context, userID uuid.UUID, date string) ([]models.Application, error) {
	var apps []models.Application
	query := `
        SELECT * FROM applications 
        WHERE user_id = $1 AND DATE(applied_at) = $2
        ORDER BY applied_at DESC
    `

	err := d.db.SelectContext(ctx, &apps, query, userID, date)
	if err != nil {
		return nil, err
	}

	return apps, nil
}

// UpdateApplication обновляет отклик
func (d *Database) UpdateApplication(ctx context.Context, app *models.Application) error {
	query := `
        UPDATE applications 
        SET status = :status, hh_application_id = :hh_application_id, 
            error_message = :error_message, updated_at = :updated_at
        WHERE id = :id
    `

	_, err := d.db.NamedExecContext(ctx, query, app)
	return err
}

// Vacancy operations

// IsVacancyProcessed проверяет, была ли обработана вакансия
func (d *Database) IsVacancyProcessed(ctx context.Context, userID uuid.UUID, vacancyID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM processed_vacancies WHERE user_id = $1 AND vacancy_id = $2`

	err := d.db.GetContext(ctx, &count, query, userID, vacancyID)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// MarkVacancyProcessed помечает вакансию как обработанную
func (d *Database) MarkVacancyProcessed(ctx context.Context, userID uuid.UUID, vacancyID string) error {
	query := `
        INSERT INTO processed_vacancies (id, user_id, vacancy_id, status, created_at, updated_at)
        VALUES ($1, $2, $3, 'applied', NOW(), NOW())
        ON CONFLICT (user_id, vacancy_id) DO UPDATE SET
            status = EXCLUDED.status,
            updated_at = EXCLUDED.updated_at
    `

	_, err := d.db.ExecContext(ctx, query, uuid.New(), userID, vacancyID)
	return err
}

// SearchSettings operations

// SaveSearchSettings сохраняет настройки поиска
func (d *Database) SaveSearchSettings(ctx context.Context, settings *models.SearchSettings) error {
	query := `
        INSERT INTO search_settings (id, user_id, positions, salary_min, salary_max, 
                                    area_id, experience, employment, schedule, 
                                    keywords, exclude_words, companies, exclude_companies,
                                    created_at, updated_at)
        VALUES (:id, :user_id, :positions, :salary_min, :salary_max, 
                :area_id, :experience, :employment, :schedule, 
                :keywords, :exclude_words, :companies, :exclude_companies,
                :created_at, :updated_at)
        ON CONFLICT (user_id) DO UPDATE SET
            positions = EXCLUDED.positions,
            salary_min = EXCLUDED.salary_min,
            salary_max = EXCLUDED.salary_max,
            area_id = EXCLUDED.area_id,
            experience = EXCLUDED.experience,
            employment = EXCLUDED.employment,
            schedule = EXCLUDED.schedule,
            keywords = EXCLUDED.keywords,
            exclude_words = EXCLUDED.exclude_words,
            companies = EXCLUDED.companies,
            exclude_companies = EXCLUDED.exclude_companies,
            updated_at = EXCLUDED.updated_at
    `

	_, err := d.db.NamedExecContext(ctx, query, settings)
	return err
}

// GetUserSearchSettings получает настройки поиска пользователя
func (d *Database) GetUserSearchSettings(ctx context.Context, userID uuid.UUID) (*models.SearchSettings, error) {
	var settings models.SearchSettings
	query := `SELECT * FROM search_settings WHERE user_id = $1`

	err := d.db.GetContext(ctx, &settings, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Возвращаем настройки по умолчанию
			return &models.SearchSettings{
				ID:         uuid.New(),
				UserID:     userID,
				Positions:  []string{},
				SalaryMin:  0,
				SalaryMax:  0,
				AreaID:     "1", // Москва по умолчанию
				Experience: "noExperience",
				Employment: "full",
				Schedule:   "fullDay",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}, nil
		}
		return nil, err
	}

	return &settings, nil
}

// UpdateSearchSettings обновляет настройки поиска
func (d *Database) UpdateSearchSettings(ctx context.Context, settings *models.SearchSettings) error {
	query := `
        UPDATE search_settings 
        SET positions = :positions, salary_min = :salary_min, salary_max = :salary_max,
            area_id = :area_id, experience = :experience, employment = :employment,
            schedule = :schedule, keywords = :keywords, exclude_words = :exclude_words,
            companies = :companies, exclude_companies = :exclude_companies,
            updated_at = :updated_at
        WHERE id = :id
    `

	_, err := d.db.NamedExecContext(ctx, query, settings)
	return err
}

// Transaction поддержка транзакций
func (d *Database) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return d.db.BeginTxx(ctx, nil)
}

// HealthCheck проверка здоровья БД
func (d *Database) HealthCheck(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// ExecContext выполняет SQL запрос без возврата строк
func (d *Database) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}
