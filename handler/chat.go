package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"vfp/repo"
)

type Message struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

type Handler struct {
	Repo *repo.Repository
}

func New(repo *repo.Repository) *Handler {
	return &Handler{Repo: repo}
}

func (h *Handler) SendHandlers(w http.ResponseWriter, r *http.Request) {
	slog.Info("send handler invoked", "method", r.Method, "path", r.URL.Path)
	if r.Method != http.MethodPost {
		slog.Warn("send handler rejected method", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	contentType := r.Header.Get("Content-Type")
	slog.Debug("send handler content type received", "content_type", contentType)

	if strings.HasPrefix(contentType, "application/json") {
		slog.Debug("decoding JSON request body")
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			slog.Error("failed to decode JSON body", "error", err)
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		slog.Debug("parsing form request body")
		err := r.ParseForm()
		if err != nil {
			slog.Error("failed to parse form body", "error", err)
			http.Error(w, "Invalid form body", http.StatusBadRequest)
			return
		}
		msg.User = r.FormValue("user")
		msg.Message = r.FormValue("message")
	}

	if msg.User == "" || msg.Message == "" {
		slog.Warn("send handler validation failed",
			"user_present", msg.User != "",
			"message_present", msg.Message != "",
		)
		http.Error(w, "Missing user or message", http.StatusBadRequest)
		return
	}

	slog.Info("saving message",
		"user", msg.User,
		"message_length", len(msg.Message),
	)
	if err := h.Repo.SaveMessage(r.Context(), msg.User, msg.Message); err != nil {
		slog.Error("failed to save message", "user", msg.User, "error", err)
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	slog.Info("message saved successfully", "user", msg.User)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Message saved"))
}

func (h *Handler) GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("get message handler invoked", "method", r.Method, "path", r.URL.Path)
	if r.Method != http.MethodGet {
		slog.Warn("get message handler rejected method", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, message, err := h.Repo.GetMessage(r.Context())
	if err != nil {
		slog.Error("failed to retrieve message", "error", err)
		http.Error(w, "Failed to retrieve message", http.StatusInternalServerError)
		return
	}
	slog.Info("message retrieved successfully", "user", user, "message_length", len(message))

	response := Message{
		User:    user,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
