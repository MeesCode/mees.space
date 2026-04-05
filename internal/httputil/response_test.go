package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	JSONError(rr, "something went wrong", http.StatusBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["error"] != "something went wrong" {
		t.Errorf("error = %q, want %q", body["error"], "something went wrong")
	}
}

func TestJSONErrorVariousCodes(t *testing.T) {
	codes := []int{
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusConflict,
	}

	for _, code := range codes {
		rr := httptest.NewRecorder()
		JSONError(rr, "test", code)
		if rr.Code != code {
			t.Errorf("status = %d, want %d", rr.Code, code)
		}
	}
}
