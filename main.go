package main

import (
	"bufio"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"vfp/handler"
	"vfp/repo"

	_ "github.com/lib/pq"
)

type server struct {
	Repo   *repo.Repository
	router *http.ServeMux
	h      *handler.Handler
}

func CROSHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("applying CORS headers", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			slog.Info("handled preflight request", "method", r.Method, "path", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		slog.Info("request started",
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"user_agent", r.UserAgent(),
		)
		next.ServeHTTP(w, r)
		slog.Info("request finished",
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

func (s *server) routes() http.Handler {
	mux := s.router
	slog.Info("registering routes")
	mux.HandleFunc("/send", s.h.SendHandlers)
	slog.Info("route registered", "method", "POST", "path", "/send")
	mux.HandleFunc("/get", s.h.GetMessageHandler)
	slog.Info("route registered", "method", "GET", "path", "/get")
	return CROSHeadersMiddleware(Middleware(mux))
}

func initDB() *sql.DB {
	slog.Info("initializing database connection")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	slog.Info("database connection settings resolved",
		"source", "DATABASE_URL",
		"database_url_present", true,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	slog.Info("sql.Open completed")
	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}
	slog.Info("database ping successful")
	ensureMessagesTable(db)
	return db
}
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
	slog.Info("application boot started")

	loadDotEnv(".env")

	db := initDB()
	defer db.Close()
	slog.Info("database connection ready")

	repo := repo.NewRepository(db)
	slog.Info("repository initialized")
	srv := &server{
		Repo:   repo,
		router: http.NewServeMux(),
		h:      handler.New(repo),
	}
	slog.Info("server struct initialized")

	port := ":" + serverPort()
	slog.Info("server port resolved", "port", port)

	slog.Info("Starting server on " + port)
	log.Fatal(http.ListenAndServe(port, srv.routes()))
}

func serverPort() string {
	if port := os.Getenv("SERVER_PORT"); port != "" {
		return port
	}
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug(".env file not found")
			return
		}
		log.Fatalf("Error opening .env file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			log.Fatalf("Error setting env %s: %v", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading .env file: %v", err)
	}
}

func ensureMessagesTable(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			message TEXT NOT NULL
		)`,
		`CREATE SEQUENCE IF NOT EXISTS messages_id_seq OWNED BY messages.id`,
		`SELECT setval('messages_id_seq', COALESCE((SELECT MAX(id) FROM messages), 0) + 1, false)`,
		`ALTER TABLE messages ALTER COLUMN id SET DEFAULT nextval('messages_id_seq')`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
		`UPDATE messages SET created_at = NOW() WHERE created_at IS NULL`,
		`ALTER TABLE messages ALTER COLUMN created_at SET DEFAULT NOW()`,
		`ALTER TABLE messages ALTER COLUMN created_at SET NOT NULL`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Error ensuring messages table: %v", err)
		}
	}
	slog.Info("messages table ready")
}
