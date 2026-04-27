package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"vfp/handler"
	"vfp/repo"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
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

	dsn := viper.GetString("database.url")
	dsnSource := "database.url"
	if dsn == "" {
		dsnSource = "database host/port/user/password/name fields"
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			viper.GetString("database.host"),
			viper.GetString("database.port"),
			viper.GetString("database.user"),
			viper.GetString("database.password"),
			viper.GetString("database.name"),
		)
	}

	slog.Info("database connection settings resolved",
		"source", dsnSource,
		"host", viper.GetString("database.host"),
		"port", viper.GetString("database.port"),
		"user", viper.GetString("database.user"),
		"name", viper.GetString("database.name"),
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
	return db
}
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
	slog.Info("application boot started")

	viper.SetConfigFile("config/config.yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	slog.Info("viper configured", "config_file", "config/config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	slog.Info("config loaded successfully")

	if viper.IsSet("database.url") {
		slog.Info("database.url is configured")
	} else {
		slog.Warn("database.url is not configured, fallback fields will be used")
	}

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

	port := ":" + viper.GetString("server.port")
	slog.Info("server port resolved", "port", port)

	slog.Info("Starting server on " + port)
	log.Fatal(http.ListenAndServe(port, srv.routes()))
}
