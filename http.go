package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/cors"
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("Request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		logger.Printf("Response: %s", w.Header().Get("Status"))
	})
}

func handleHTTP(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/info", getInfoFromDB)
	mux.HandleFunc("/connections", getConnections)
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("/deleteOldEntries", func(w http.ResponseWriter, r *http.Request) {
		db, err := openDatabase()
		if err != nil {
			logger.Printf("Error opening database: %v", err)
			http.Error(w, "Failed to open database", http.StatusInternalServerError)
			return
		}
		defer db.Close()

		err = deleteOldEntries(db)
		if err != nil {
			logger.Printf("Error deleting old entries: %v", err)
			http.Error(w, "Failed to delete old entries", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "old entries deleted"})
	})

	mux.HandleFunc("/truncateDatabase", func(w http.ResponseWriter, r *http.Request) {
		db, err := openDatabase()
		if err != nil {
			logger.Printf("Error opening database: %v", err)
			http.Error(w, "Failed to open database", http.StatusInternalServerError)
			return
		}
		defer db.Close()

		err = truncateDatabase(db)
		if err != nil {
			logger.Printf("Error truncating database: %v", err)
			http.Error(w, "Failed to truncate database", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "database truncated"})
	})

	// Create a new CORS middleware
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // Allow all origins
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		MaxAge:         300, // Maximum age (in seconds) of the preflight request
	})

	// Create a handler chain: cors -> logging -> mux
	handler := corsMiddleware.Handler(loggingMiddleware(mux))

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: handler,
	}

	// Start the server
	go func() {
		logger.Printf("Server is running on port %d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to run server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	logger.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server exiting")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Handler to get info from the database
func getInfoFromDB(w http.ResponseWriter, r *http.Request) {
	db, err := openDatabase()
	if err != nil {
		logger.Printf("Error opening database: %v", err)
		http.Error(w, "Failed to open database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20 // Default limit
	}

	offset := (page - 1) * limit

	// Update query with pagination
	rows, err := db.Query("SELECT file_name, file_size, download_time, server_name FROM downloaded_files ORDER BY download_time DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		logger.Printf("Error querying database: %v", err)
		http.Error(w, "Failed to query database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var files []DownloadedFile
	for rows.Next() {
		var file DownloadedFile
		if err := rows.Scan(&file.FileName, &file.FileSize, &file.DownloadTime, &file.ServerName); err != nil {
			logger.Printf("Error scanning row: %v", err)
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		files = append(files, file)
	}

	// Get total count
	var totalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM downloaded_files").Scan(&totalCount)
	if err != nil {
		logger.Printf("Error getting total count: %v", err)
		http.Error(w, "Failed to get total count", http.StatusInternalServerError)
		return
	}

	response := struct {
		Files      []DownloadedFile `json:"files"`
		TotalCount int              `json:"totalCount"`
		Page       int              `json:"page"`
		Limit      int              `json:"limit"`
	}{
		Files:      files,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Handler to get connections from the YAML configuration
func getConnections(w http.ResponseWriter, r *http.Request) {
	config, err := readConfig("connections.yaml")
	if err != nil {
		http.Error(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.Connections)
}
