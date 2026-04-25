package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type Message struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

func TestSendMessage(t *testing.T) {

	messages := []Message{}

	msg := Message{
		User:    "testuser",
		Message: "Hello, World!",
	}
	jsonData, _ := json.Marshal(msg)

	req, err := http.NewRequest("POST", "/send", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m Message
		json.NewDecoder(r.Body).Decode(&m)
		messages = append(messages, m)
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if len(messages) != 1 {
		t.Errorf("Сообщение не сохранилось в памяти")
	}

}

func TestGetMessage(t *testing.T) {

	messages := []Message{{User: "Admin", Message: "Test message"}}

	req, err := http.NewRequest("GET", "/messages", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(messages)
	})

	handler.ServeHTTP(rr, req)

	var res []Message
	err = json.NewDecoder(rr.Body).Decode(&res)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	if len(res) == 0 || res[0].User != "Admin" {
		t.Errorf("response does not contain expected message")
	}
}
