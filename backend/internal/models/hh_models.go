package models

import (
	"time"
)

// HHVacancy вакансия с HH.ru
type HHVacancy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`

	Employer struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"employer"`

	Salary *Salary `json:"salary"`
	Area   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"area"`

	Experience struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"experience"`

	Employment struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"employment"`

	Schedule struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"schedule"`

	KeySkills []struct {
		Name string `json:"name"`
	} `json:"key_skills"`

	Contacts struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"contacts"`

	Type struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"type"`

	ResponseURL    string `json:"response_url"`
	HasTest        bool   `json:"has_test"`
	ResponseLetter string `json:"response_letter_required"`
}

// Salary информация о зарплате
type Salary struct {
	From     int    `json:"from"`
	To       int    `json:"to"`
	Currency string `json:"currency"`
	Gross    bool   `json:"gross"`
}

// HHResume резюме пользователя на HH.ru
type HHResume struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	AccessType struct {
		Type string `json:"type"`
	} `json:"access"`

	Contacts struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"contacts"`

	Experience []struct {
		Position    string `json:"position"`
		Company     string `json:"company"`
		StartDate   string `json:"start"`
		EndDate     string `json:"end"`
		Description string `json:"description"`
	} `json:"experience"`

	Skills []struct {
		Name string `json:"name"`
	} `json:"skills"`

	Language []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"language"`
}

// HHUserInfo информация о пользователе с HH.ru
type HHUserInfo struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	MiddleName  string `json:"middle_name"`
	Email       string `json:"email"`
	IsEmployer  bool   `json:"is_employer"`
	IsApplicant bool   `json:"is_applicant"`

	Negotiations struct {
		Total  int `json:"total"`
		Unread int `json:"unread"`
	} `json:"negotiations"`

	Resumes []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"resumes"`
}
