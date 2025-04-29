package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Config struct {
	Port    string
	Message string
}

func main() {
	config := setupConfig()
	setupLogging(config)
	setupHandlers(config)
	startServer(config)
}

func setupConfig() *Config {
	config := &Config{}

	port := flag.String("port", "8080", "port")
	message := flag.String("message", "Hello", "message")
	flag.Parse()

	config.Port = *port
	config.Message = *message

	if envPort := os.Getenv("SERVER_PORT"); envPort != "" {
		config.Port = envPort
	}

	if envMessage := os.Getenv("SERVER_MESSAGE"); envMessage != "" {
		config.Message = envMessage
	}

	fmt.Printf("Server config: port=%s, message=%s\n", config.Port, config.Message)

	return config
}

func setupLogging(config *Config) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix(fmt.Sprintf("[Server:%s] ", config.Port))
}

func setupHandlers(config *Config) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, config)
	})
	http.HandleFunc("/health", handleHealth)
}

func startServer(config *Config) {
	addr := fmt.Sprintf(":%s", config.Port)
	log.Printf("Starting server on %s", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request, config *Config) {
	log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	response := buildResponse(r, body, config)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)
}

func buildResponse(r *http.Request, body []byte, config *Config) string {
	response := fmt.Sprintf("Server Message: %s\n\nRequest Details:\nMethod: %s\nPath: %s\nHeaders:\n",
		config.Message, r.Method, r.URL.Path)

	for name, values := range r.Header {
		for _, value := range values {
			response += fmt.Sprintf("%s: %s\n", name, value)
		}
	}

	if len(body) > 0 {
		response += fmt.Sprintf("\nRequest Body:\n%s", string(body))
	}

	return response
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status": "ok"}`)
}
