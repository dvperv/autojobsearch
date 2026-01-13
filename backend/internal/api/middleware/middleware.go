package middleware

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Middleware цепочка middleware
type Middleware func(http.Handler) http.Handler

// Chain создает цепочку middleware
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Apply применяет middleware к handler
func Apply(h http.Handler, middlewares ...Middleware) http.Handler {
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}

// AuthRequiredMiddleware проверяет аутентификацию
func AuthRequiredMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверка аутентификации
		userID := GetUserIDFromContext(r.Context())
		if userID == uuid.Nil {
			http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// APIKeyMiddleware middleware для проверки API ключа (для внутренних сервисов)
func APIKeyMiddleware(apiKey string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get("X-API-Key")
			if providedKey != apiKey {
				http.Error(w, `{"error": "Invalid API key"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware добавляет ID запроса
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(r.Context(), "request_id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CompressionMiddleware middleware для сжатия ответов
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем поддержку сжатия
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// Создаем gzip writer
			gz := gzip.NewWriter(w)
			defer gz.Close()

			// Устанавливаем заголовки
			w.Header().Set("Content-Encoding", "gzip")

			// Обертываем ResponseWriter
			gzw := gzipResponseWriter{ResponseWriter: w, Writer: gz}
			next.ServeHTTP(gzw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// gzipResponseWriter кастомный ResponseWriter для gzip
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (grw gzipResponseWriter) Write(b []byte) (int, error) {
	return grw.Writer.Write(b)
}

// TimeoutMiddleware устанавливает таймаут для запросов
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Создаем канал для отслеживания завершения
			done := make(chan struct{})
			panicChan := make(chan interface{}, 1)

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
					close(done)
				}()

				next.ServeHTTP(w, r.WithContext(ctx))
				done <- struct{}{}
			}()

			select {
			case <-panicChan:
				// Обработка паники
				http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
			case <-done:
				// Запрос завершен успешно
				return
			case <-ctx.Done():
				// Таймаут
				http.Error(w, `{"error": "Request timeout"}`, http.StatusRequestTimeout)
			}
		})
	}
}
