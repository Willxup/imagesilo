package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestRemoteAddressUsesNginxProxyHeadersInPriorityOrder(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("X-Real-IP", "198.51.100.8")
	request.Header.Set("X-Forwarded-For", "192.0.2.1, 203.0.113.9")
	if got := remoteAddress(request, true); got != "198.51.100.8" {
		t.Fatalf("remoteAddress() = %q, want X-Real-IP", got)
	}

	request.Header.Del("X-Real-IP")
	request.Header.Add("X-Real-IP", "198.51.100.8")
	request.Header.Add("X-Real-IP", "198.51.100.9")
	request.Header.Set("X-Forwarded-For", "192.0.2.1, invalid, 203.0.113.9")
	if got := remoteAddress(request, true); got != "203.0.113.9" {
		t.Fatalf("remoteAddress() = %q, want right-most valid X-Forwarded-For", got)
	}
}

func TestRemoteAddressCanIgnoreProxyHeadersForDirectDeployment(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("X-Real-IP", "198.51.100.8")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := remoteAddress(request, false); got != "127.0.0.1" {
		t.Fatalf("remoteAddress() = %q, want direct peer", got)
	}
}
