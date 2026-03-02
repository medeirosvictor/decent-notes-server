package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

//go:embed static/*
var staticFiles embed.FS

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

func entriesHandler(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			entries, err := db.GetAllEntries()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entries)

		case http.MethodPost:
			var entry Entry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			entry.CreatedAt = time.Now()
			entry.UpdatedAt = time.Now()
			if entry.ID == "" {
				entry.ID = uuid.New().String()
			}
			// Todos are title-only — clear description
			if entry.Type == TypeTodo {
				entry.Description = ""
				entry.IsHabit = false
				entry.HabitData = nil
			}
			// Only notes can be habits
			if entry.Type == TypeNote && entry.IsHabit && entry.HabitData == nil {
				entry.HabitData = &HabitData{RepeatDays: []string{}, RepeatHour: "06:00"}
			}
			if err := db.CreateEntry(&entry); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(entry)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func entryHandler(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		switch r.Method {
		case http.MethodGet:
			entry, err := db.GetEntry(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entry)

		case http.MethodPut:
			var entry Entry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			entry.ID = id
			entry.UpdatedAt = time.Now()
			// Todos are title-only
			if entry.Type == TypeTodo {
				entry.Description = ""
				entry.IsHabit = false
				entry.HabitData = nil
			}
			if err := db.UpdateEntry(&entry); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entry)

		case http.MethodDelete:
			if err := db.DeleteEntry(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

const infoLogFile = "decent-notes-info.log"

// checkPort tests whether a port is available. Returns nil if free.
func checkPort(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}

// findFreePort scans upward from start to find an available port.
func findFreePort(start int) (int, bool) {
	for p := start; p <= start+100; p++ {
		if checkPort(strconv.Itoa(p)) == nil {
			return p, true
		}
	}
	return 0, false
}

// writeInfoLog writes a message to the info log file (overwriting previous content).
func writeInfoLog(msg string) {
	_ = os.WriteFile(infoLogFile, []byte(msg), 0644)
}

func main() {
	port := getEnv("PORT", "5050")
	dbFile := getEnv("DB_FILE", "decent-notes.db")

	// Check if the requested port is available before doing anything else
	if err := checkPort(port); err != nil {
		portNum, _ := strconv.Atoi(port)
		suggested, found := findFreePort(portNum + 1)

		msg := fmt.Sprintf("[%s] ERROR: Port %s is already in use.\n", time.Now().Format(time.RFC3339), port)
		if found {
			msg += fmt.Sprintf("  Suggested available port: %d\n", suggested)
			msg += fmt.Sprintf("  To use it, run:  PORT=%d go run .\n", suggested)
			msg += fmt.Sprintf("  Or with Docker:  docker run -e PORT=%d -p %d:%d ...\n", suggested, suggested, suggested)
		} else {
			msg += "  Could not find a free port nearby. Check your system for available ports.\n"
		}
		writeInfoLog(msg)
		log.Fatalf("Port %s is already in use. See %s for details.", port, infoLogFile)
	}

	fmt.Println("Decent Notes Server starting on port " + port)

	db, err := NewDB(dbFile)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	fmt.Println("Database initialized: " + dbFile)

	mux := http.NewServeMux()

	corsHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/health", corsHandler(healthHandler))
	mux.HandleFunc("/entries", corsHandler(entriesHandler(db)))
	mux.HandleFunc("/entries/{id}", corsHandler(entryHandler(db)))

	// Serve embedded web app (built into static/ at build time)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load embedded static files: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file; if it doesn't exist, serve index.html (SPA fallback)
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Check if file exists in embedded FS
		if _, err := fs.Stat(staticFS, path[1:]); err != nil {
			// File not found — serve index.html for SPA client-side routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := ":" + port
	fmt.Printf("Server listening on http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
