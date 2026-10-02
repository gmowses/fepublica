package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestExpensiveRouteLimitsConcurrentRequests(t *testing.T) {
	s := &Server{expensive: make(chan struct{}, 2)}
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	handler := s.expensiveRoute(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { entered <- struct{}{}; <-release }))
	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		}()
	}
	<-entered
	<-entered
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After=%q, want 5", got)
	}
	release <- struct{}{}
	release <- struct{}{}
	wg.Wait()
}
