package main

import (
	"autojobsearch-backend/internal/proxy"
	"log"
	"net/http"
)

func main() {
	proxyHandler := proxy.NewHandler()

	http.HandleFunc("/proxy/hh/", proxyHandler.HandleRequest)

	log.Println("Starting secure proxy server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server failed:", err)
	}
}
