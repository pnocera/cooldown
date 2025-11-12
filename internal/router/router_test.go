package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRouter(t *testing.T) {
	// Create target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Routed-To", r.Host)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	routes := map[string]*url.URL{
		"api.github.com": mustParse(targetServer.URL),
	}

	router := New(routes, nil)
	
	req := httptest.NewRequest("GET", "http://api.github.com/users/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	resp := w.Result()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestRouterNotFound(t *testing.T) {
	routes := map[string]*url.URL{}
	router := New(routes, nil)
	
	req := httptest.NewRequest("GET", "http://unknown.host/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	resp := w.Result()
	
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestRouterDirectPassthrough(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original-Host", r.Host)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	// Add route for the target server
	routes := map[string]*url.URL{
		"test-server": mustParse(targetServer.URL),
	}
	router := New(routes, nil)
	
	// Create request that should route to target
	req := httptest.NewRequest("GET", "http://test-server/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	resp := w.Result()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}