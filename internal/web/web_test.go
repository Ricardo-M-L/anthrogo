package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func handler(t *testing.T) http.Handler {
	t.Helper()
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler() error: %v", err)
	}
	return h
}

func TestWeb_Handler_ServesIndex(t *testing.T) {
	h := handler(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content-type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<title>") {
		t.Fatalf("expected <title> in body; body starts: %s", body[:min(200, len(body))])
	}
}

func TestWeb_Handler_ServesAppJS(t *testing.T) {
	h := handler(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Fatalf("expected javascript content-type, got %q", ct)
	}
}

func TestWeb_Handler_ServesCSS(t *testing.T) {
	h := handler(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "css") {
		t.Fatalf("expected css content-type, got %q", ct)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
