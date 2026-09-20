package main

import "testing"

func TestParseOverview(t *testing.T) {
	raw := []byte(`<html><body><span class="info-box-number">12</span><span class="info-box-number">1,234</span><span class="info-box-number">3</span></body></html>`)
	result, err := parseOverview(raw)
	if err != nil || result["jobs"] != 12 || result["triggers"] != 1234 || result["executors"] != 3 {
		t.Fatalf("overview: %v %v", result, err)
	}
	if _, err := parseOverview([]byte(`<html><body>login</body></html>`)); err == nil {
		t.Fatal("accepted missing dashboard")
	}
}
