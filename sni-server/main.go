package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	port   int
	secret string
)

func main() {
	flag.IntVar(&port, "port", 443, "Listen port")
	flag.StringVar(&secret, "secret", "", "Auth secret (x-snishaper-auth)")
	flag.Parse()

	// 优先从环境变量读取 Secret
	if envSecret := os.Getenv("AUTH_SECRET"); envSecret != "" {
		secret = envSecret
	}

	if secret == "" {
		log.Fatal("Auth secret is required. Set via -secret flag or AUTH_SECRET environment variable.")
	}

	proxy := NewProxy(secret)

	serverAddr := fmt.Sprintf(":%d", port)
	log.Printf("Snishaper Server (sni-server) starting on %s", serverAddr)

	err := http.ListenAndServe(serverAddr, proxy)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
