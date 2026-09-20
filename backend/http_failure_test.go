package main

import (
	"strings"
	"testing"
)

func TestHTTPFailureDetail(t *testing.T) {
	got := httpFailureDetail(400, "POST", "/user/add", []byte(`{"msg":"username already exists"}`))
	if !strings.Contains(got, "HTTP 400 · POST /user/add") || !strings.Contains(got, "username already exists") {
		t.Fatalf("missing status, endpoint or server detail: %q", got)
	}
	got = httpFailureDetail(400, "POST", "/user/add", []byte(`<html>server error</html>`))
	if strings.Contains(got, "server error") || !strings.Contains(got, "HTTP 400") {
		t.Fatalf("HTML response should not enter the detail message: %q", got)
	}
}
