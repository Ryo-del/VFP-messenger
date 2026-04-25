package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

func (s *server) routes() http.Handler {
	mux := s.router
	mux.HandleFunc("/send", s.h.SendHandlers)
	mux.HandleFunc("/get", s.h.GetMessageHandler)
	return CROSHeadersMiddleware(Middleware(mux))
}

func initDB() *sql.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		viper.GetString("database.host"),
		viper.GetString("database.port"),
		viper.GetString("database.user"),
		viper.GetString("database.password"),
		viper.GetString("database.name"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}
	return db
}
func main() {

	viper.SetConfigFile("config/config.yaml")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	db := initDB()
	defer db.Close()

	repo := repo.NewRepository(db)
	srv := &server{
		Repo:   repo,
		router: http.NewServeMux(),
	}

	port := ":" + viper.GetString("server.port")

	slog.Info("Starting server on " + port)
	log.Fatal(http.ListenAndServe(port, srv.routes()))
}
