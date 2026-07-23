package main

import ("net/http"; "net/http/httptest"; "testing")

func TestHealthz(t *testing.T) {
 request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
 response := httptest.NewRecorder()
 healthz(response, request)
 if response.Code != http.StatusOK { t.Fatalf("status = %d", response.Code) }
}
