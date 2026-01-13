package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Handler struct {
	allowedEndpoints map[string]bool
}

func NewHandler() *Handler {
	return &Handler{
		allowedEndpoints: map[string]bool{
			"vacancies":    true,
			"negotiations": true,
			"resumes":      true,
			"employers":    true,
		},
	}
}

func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Извлечь токен пользователя из заголовка
	userToken := r.Header.Get("X-HH-Access-Token")
	if userToken == "" {
		http.Error(w, "Access token required", http.StatusBadRequest)
		return
	}

	// 2. Извлечь endpoint из URL
	path := strings.TrimPrefix(r.URL.Path, "/proxy/hh/")
	endpoint := strings.Split(path, "/")[0]

	// 3. Проверить разрешенный endpoint
	if !h.allowedEndpoints[endpoint] {
		http.Error(w, "Endpoint not allowed", http.StatusForbidden)
		return
	}

	// 4. Создать запрос к HH.ru
	hhURL := fmt.Sprintf("https://api.hh.ru/%s?%s", path, r.URL.RawQuery)
	proxyReq, err := http.NewRequest(r.Method, hhURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// 5. Установить заголовки (только необходимые)
	proxyReq.Header.Set("Authorization", "Bearer "+userToken)
	proxyReq.Header.Set("User-Agent", r.Header.Get("User-Agent"))
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	// 6. Выполнить запрос
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to reach HH.ru", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 7. Скопировать ответ клиенту
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
