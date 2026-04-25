package handler

import (
	"encoding/json"
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Invalid form body", http.StatusBadRequest)
			return
		}
		msg.User = r.FormValue("user")
		msg.Message = r.FormValue("message")
	}

	if msg.User == "" || msg.Message == "" {
		http.Error(w, "Missing user or message", http.StatusBadRequest)
		return
	}

	if err := h.Repo.SaveMessage(r.Context(), msg.User, msg.Message); err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Message saved"))
}

func (h *Handler) GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, message, err := h.Repo.GetMessage(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve message", http.StatusInternalServerError)
		return
	}

	response := Message{
		User:    user,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
