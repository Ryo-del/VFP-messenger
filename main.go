package main

import (
	"database/sql"
	"errors"
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

type dbConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func loadDBConfig() dbConfig {
	cfg := dbConfig{
		Host:     viper.GetString("database.host"),
		Port:     viper.GetString("database.port"),
		User:     viper.GetString("database.user"),
		Password: viper.GetString("database.password"),
		Name:     viper.GetString("database.name"),
		SSLMode:  viper.GetString("database.sslmode"),
	}

	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	return cfg
}

func validateDBConfig(cfg dbConfig) error {
	missing := make([]string, 0, 5)
	if cfg.Host == "" {
		missing = append(missing, "DATABASE_HOST")
	}
	if cfg.Port == "" {
		missing = append(missing, "DATABASE_PORT")
	}
	if cfg.User == "" {
		missing = append(missing, "DATABASE_USER")
	}
	if cfg.Name == "" {
		missing = append(missing, "DATABASE_NAME")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required database config: %s", strings.Join(missing, ", "))
	}

	return nil
}

func initDB() *sql.DB {
	slog.Info("initializing database connection")

	cfg := loadDBConfig()
	if err := validateDBConfig(cfg); err != nil {
		log.Fatal(err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	slog.Info("database connection settings resolved",
		"host", cfg.Host,
		"port", cfg.Port,
		"user", cfg.User,
		"name", cfg.Name,
		"sslmode", cfg.SSLMode,
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

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "password")
	viper.SetDefault("database.name", "postgres")
	viper.SetDefault("database.sslmode", "disable")

	viper.SetConfigFile("config/config.yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	mustBindEnv("server.port", "SERVER_PORT")
	mustBindEnv("database.host", "DATABASE_HOST")
	mustBindEnv("database.port", "DATABASE_PORT")
	mustBindEnv("database.user", "DATABASE_USER")
	mustBindEnv("database.password", "DATABASE_PASSWORD")
	mustBindEnv("database.name", "DATABASE_NAME")
	mustBindEnv("database.sslmode", "DATABASE_SSLMODE")
	slog.Info("viper configured", "config_file", "config/config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			slog.Warn("config file not found, using env/default values", "config_file", "config/config.yaml")
		} else {
			log.Fatalf("Error reading config file: %v", err)
		}
	} else {
		slog.Info("config loaded successfully")
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

func mustBindEnv(key, envKey string) {
	if err := viper.BindEnv(key, envKey); err != nil {
		log.Fatalf("Error binding env %s to %s: %v", envKey, key, err)
	}
}
