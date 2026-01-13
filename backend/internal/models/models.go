package models

import (
	"time"

	"github.com/google/uuid"
)

// User пользователь системы
type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Дополнительные поля
	Phone     *string      `json:"phone,omitempty" db:"phone"`
	AvatarURL *string      `json:"avatar_url,omitempty" db:"avatar_url"`
	Settings  UserSettings `json:"settings" db:"settings"`
}

// UserSettings настройки пользователя
type UserSettings struct {
	EmailNotifications bool   `json:"email_notifications" db:"email_notifications"`
	PushNotifications  bool   `json:"push_notifications" db:"push_notifications"`
	Language           string `json:"language" db:"language"`
	Theme              string `json:"theme" db:"theme"`
}

// SearchSettings настройки поиска
type SearchSettings struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Positions  []string  `json:"positions" db:"positions"`
	SalaryMin  int       `json:"salary_min" db:"salary_min"`
	SalaryMax  int       `json:"salary_max" db:"salary_max"`
	AreaID     string    `json:"area_id" db:"area_id"`
	Experience string    `json:"experience" db:"experience"`
	Employment string    `json:"employment" db:"employment"`
	Schedule   string    `json:"schedule" db:"schedule"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`

	// Дополнительные фильтры
	Keywords         []string `json:"keywords,omitempty" db:"keywords"`
	ExcludeWords     []string `json:"exclude_words,omitempty" db:"exclude_words"`
	Companies        []string `json:"companies,omitempty" db:"companies"`
	ExcludeCompanies []string `json:"exclude_companies,omitempty" db:"exclude_companies"`
}

// Resume резюме пользователя
type Resume struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Title      string     `json:"title" db:"title"`
	FilePath   string     `json:"file_path" db:"file_path"`
	FileType   string     `json:"file_type" db:"file_type"`
	FileSize   int64      `json:"file_size" db:"file_size"`
	ParsedData ResumeData `json:"parsed_data" db:"parsed_data"`
	IsPrimary  bool       `json:"is_primary" db:"is_primary"`
	HHResumeID *string    `json:"hh_resume_id,omitempty" db:"hh_resume_id"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}

// ResumeData распарсенные данные резюме
type ResumeData struct {
	FirstName       string           `json:"first_name"`
	LastName        string           `json:"last_name"`
	Email           string           `json:"email"`
	Phone           string           `json:"phone,omitempty"`
	Location        string           `json:"location,omitempty"`
	Title           string           `json:"title,omitempty"`
	Summary         string           `json:"summary,omitempty"`
	TotalExperience int              `json:"total_experience"` // в годах
	DesiredSalary   int              `json:"desired_salary,omitempty"`
	Skills          []string         `json:"skills"`
	Experience      []ExperienceItem `json:"experience"`
	Education       []EducationItem  `json:"education"`
	Languages       []LanguageItem   `json:"languages"`
	Certifications  []string         `json:"certifications,omitempty"`
}

// ExperienceItem опыт работы
type ExperienceItem struct {
	Title       string     `json:"title"`
	Company     string     `json:"company"`
	Location    string     `json:"location,omitempty"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	IsCurrent   bool       `json:"is_current"`
	Description string     `json:"description,omitempty"`
}

// EducationItem образование
type EducationItem struct {
	Degree      string    `json:"degree"`
	Institution string    `json:"institution"`
	Location    string    `json:"location,omitempty"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Description string    `json:"description,omitempty"`
}

// LanguageItem знание языков
type LanguageItem struct {
	Language string `json:"language"`
	Level    string `json:"level"` // beginner, intermediate, advanced, native
}

// Application отклик на вакансию
type Application struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	VacancyID       string    `json:"vacancy_id" db:"vacancy_id"`
	VacancyTitle    string    `json:"vacancy_title" db:"vacancy_title"`
	CompanyName     string    `json:"company_name" db:"company_name"`
	ResumeID        uuid.UUID `json:"resume_id" db:"resume_id"`
	CoverLetter     string    `json:"cover_letter" db:"cover_letter"`
	Status          string    `json:"status" db:"status"` // pending, sent, viewed, rejected, accepted, duplicate, failed
	MatchScore      float64   `json:"match_score" db:"match_score"`
	AppliedAt       time.Time `json:"applied_at" db:"applied_at"`
	Automated       bool      `json:"automated" db:"automated"`
	Source          string    `json:"source" db:"source"` // hh.ru, direct, etc.
	HHApplicationID *string   `json:"hh_application_id,omitempty" db:"hh_application_id"`
	ErrorMessage    *string   `json:"error_message,omitempty" db:"error_message"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	// Ссылка на вакансию
	VacancyURL *string `json:"vacancy_url,omitempty" db:"vacancy_url"`
}

// VacancyProcessed обработанная вакансия
type VacancyProcessed struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	VacancyID string    `json:"vacancy_id" db:"vacancy_id"`
	Status    string    `json:"status" db:"status"` // seen, applied, ignored, saved
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Invitation приглашение на собеседование
type Invitation struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	ApplicationID uuid.UUID  `json:"application_id" db:"application_id"`
	CompanyName   string     `json:"company_name" db:"company_name"`
	Position      string     `json:"position" db:"position"`
	ReceivedAt    time.Time  `json:"received_at" db:"received_at"`
	InterviewDate *time.Time `json:"interview_date,omitempty" db:"interview_date"`
	Status        string     `json:"status" db:"status"` // pending, accepted, rejected, rescheduled
	Message       string     `json:"message" db:"message"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// Notification уведомление пользователя
type Notification struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id"`
	Type      string                 `json:"type" db:"type"` // automation_started, application_sent, invitation_received, etc.
	Title     string                 `json:"title" db:"title"`
	Message   string                 `json:"message" db:"message"`
	Data      map[string]interface{} `json:"data,omitempty" db:"data"`
	IsRead    bool                   `json:"is_read" db:"is_read"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

// AuditLog лог действий пользователя
type AuditLog struct {
	ID         uuid.UUID              `json:"id" db:"id"`
	UserID     uuid.UUID              `json:"user_id" db:"user_id"`
	Action     string                 `json:"action" db:"action"`
	Resource   string                 `json:"resource" db:"resource"`
	ResourceID string                 `json:"resource_id" db:"resource_id"`
	Details    map[string]interface{} `json:"details,omitempty" db:"details"`
	IPAddress  string                 `json:"ip_address" db:"ip_address"`
	UserAgent  string                 `json:"user_agent" db:"user_agent"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}
