package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/models"
)

// HHTokensStorage операции с токенами HH.ru
type HHTokensStorage struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewHHTokensStorage создает новый storage для токенов HH.ru
func NewHHTokensStorage(db *sqlx.DB, logger *zap.Logger) *HHTokensStorage {
	return &HHTokensStorage{
		db:     db,
		logger: logger,
	}
}

// SaveHHTokens сохраняет токены HH.ru
func (s *HHTokensStorage) SaveHHTokens(ctx context.Context, tokens *models.UserHHTokens) error {
	query := `
        INSERT INTO hh_tokens (user_id, access_token, refresh_token, expires_at, 
                              token_type, scope, created_at, updated_at)
        VALUES (:user_id, :access_token, :refresh_token, :expires_at, 
                :token_type, :scope, :created_at, :updated_at)
        ON CONFLICT (user_id) DO UPDATE SET
            access_token = EXCLUDED.access_token,
            refresh_token = EXCLUDED.refresh_token,
            expires_at = EXCLUDED.expires_at,
            token_type = EXCLUDED.token_type,
            scope = EXCLUDED.scope,
            updated_at = EXCLUDED.updated_at
    `

	_, err := s.db.NamedExecContext(ctx, query, tokens)
	return err
}

// GetHHTokens получает токены HH.ru
func (s *HHTokensStorage) GetHHTokens(ctx context.Context, userID uuid.UUID) (*models.UserHHTokens, error) {
	var tokens models.UserHHTokens
	query := `SELECT * FROM hh_tokens WHERE user_id = $1`

	err := s.db.GetContext(ctx, &tokens, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &tokens, nil
}

// UpdateHHTokens обновляет токены HH.ru
func (s *HHTokensStorage) UpdateHHTokens(ctx context.Context, tokens *models.UserHHTokens) error {
	query := `
        UPDATE hh_tokens 
        SET access_token = :access_token,
            refresh_token = :refresh_token,
            expires_at = :expires_at,
            token_type = :token_type,
            scope = :scope,
            updated_at = :updated_at
        WHERE user_id = :user_id
    `

	_, err := s.db.NamedExecContext(ctx, query, tokens)
	return err
}

// DeleteHHTokens удаляет токены HH.ru
func (s *HHTokensStorage) DeleteHHTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM hh_tokens WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// GetExpiredTokens получает истекшие токены
func (s *HHTokensStorage) GetExpiredTokens(ctx context.Context, before time.Time) ([]models.UserHHTokens, error) {
	var tokens []models.UserHHTokens
	query := `SELECT * FROM hh_tokens WHERE expires_at < $1`

	err := s.db.SelectContext(ctx, &tokens, query, before)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// CleanupExpiredTokens удаляет истекшие токены
func (s *HHTokensStorage) CleanupExpiredTokens(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM hh_tokens WHERE expires_at < $1`
	result, err := s.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
