package pkg

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSample(t *testing.T) {
	sampleFunc()
}

func TestRootHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/rootHandler", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(RootHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
