package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnsupportedVersionFailsBeforeSendingCredentials(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer srv.Close()
	c := officialConnection(t, srv.URL, "user", "password")
	c.Config.AdminVersion = "3.3"
	if s, err := newSession(c, runtimeEndpoint{}); err == nil {
		s.close()
		t.Fatal("unsupported version accepted")
	}
	if calls != 0 {
		t.Fatal("unsupported version sent credentials")
	}
}

func TestModernLoginPagingAndWrites(t *testing.T) {
	requests := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		if r.URL.Path == "/auth/doLogin" {
			r.ParseForm()
			if r.Form.Get("userName") != "demo" || r.Form.Get("password") != "test-only" || r.Method != "POST" {
				t.Error("modern login parameters mismatch")
			}
			http.SetCookie(w, &http.Cookie{Name: "XXL_SSO_TOKEN", Value: "modern-test", Path: "/"})
			fmt.Fprint(w, `{"code":200,"data":null}`)
			return
		}
		if cookie, err := r.Cookie("XXL_SSO_TOKEN"); err != nil || cookie.Value != "modern-test" {
			t.Error("authenticated modern request missing session")
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<html><a href="/jobgroup">Groups</a><a href="/jobinfo">Jobs</a></html>`)
			return
		}
		r.ParseForm()
		switch r.URL.Path {
		case "/jobgroup/pageList":
			if r.Form.Get("offset") != "0" || r.Form.Get("pagesize") != "100" || r.Form.Has("start") {
				t.Errorf("group pagination not adapted: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"data":{"total":1,"data":[{"id":3,"appname":"test-app","title":"Test","addressType":0,"addressList":""}]}}`)
		case "/jobinfo/pageList":
			if r.Form.Get("offset") != "0" || r.Form.Get("pagesize") != "25" || r.Form.Has("length") {
				t.Errorf("job pagination not adapted: %v", r.Form)
			}
			for _, key := range []string{"jobDesc", "executorHandler", "author"} {
				if !r.Form.Has(key) {
					t.Errorf("missing required modern job filter %s: %v", key, r.Form)
				}
			}
			fmt.Fprint(w, `{"code":200,"data":{"total":1,"data":[{"id":7,"jobGroup":3,"jobDesc":"Test"}]}}`)
		case "/joblog/pageList":
			if !r.Form.Has("filterTime") {
				t.Errorf("missing required modern log filter: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"data":{"total":1,"data":[{"id":11,"jobGroup":3,"jobId":7,"triggerTime":"2026-09-18 10:00:00","executorAddress":"http://executor/"}]}}`)
		case "/joblog/logDetailCat":
			if r.Form.Get("logId") != "11" || r.Form.Get("fromLineNum") != "1" || r.Form.Has("triggerTime") || r.Form.Has("executorAddress") {
				t.Errorf("modern log detail form mismatch: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"data":{"fromLineNum":1,"toLineNum":1,"logContent":"ok","end":true}}`)
		case "/jobinfo/start", "/jobinfo/stop", "/jobinfo/delete", "/jobgroup/delete":
			expectedID := "7"
			if r.URL.Path == "/jobgroup/delete" {
				expectedID = "3"
			}
			if r.Form.Get("ids[]") != expectedID || r.Form.Has("id") {
				t.Errorf("modern mutation form mismatch: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"data":null}`)
		case "/jobinfo/trigger":
			if r.Form.Get("id") != "7" || !r.Form.Has("executorParam") || !r.Form.Has("addressList") || r.Form.Get("addressList") != "" {
				t.Errorf("modern trigger form mismatch: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"data":null}`)
		case "/jobinfo/insert", "/jobgroup/insert":
			fmt.Fprint(w, `{"code":200,"data":null}`)
		default:
			t.Errorf("unexpected modern endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p := newPlugin()
	c := officialConnection(t, srv.URL, "demo", "test-only")
	c.Config.AdminVersion = "3.4"
	if _, err := p.handle("connection/connect", params{Connection: c}); err != nil {
		t.Fatal(err)
	}
	defer p.handle("connection/disconnect", params{Connection: connection{ID: c.ID}})
	info, err := p.handle("xxljob/info", params{ConnectionID: c.ID})
	if err != nil || info.(map[string]any)["adminVersion"] != "3.4" {
		t.Fatalf("version not exposed: %v %v", info, err)
	}
	for _, method := range []string{"xxljob/groups", "xxljob/jobs", "xxljob/logs"} {
		value, err := p.handle(method, params{ConnectionID: c.ID})
		if err != nil || value == nil {
			t.Fatalf("%s: %v %v", method, value, err)
		}
	}
	if _, err := p.handle("xxljob/logContent", params{ConnectionID: c.ID, Form: map[string]any{"logId": float64(11), "fromLineNum": float64(1)}}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"xxljob/start", "xxljob/stop", "xxljob/trigger", "xxljob/removeJob", "xxljob/removeGroup"} {
		id := 7
		if method == "xxljob/removeGroup" {
			id = 3
		}
		if _, err := p.handle(method, params{ConnectionID: c.ID, Confirmed: true, Form: map[string]any{"id": float64(id)}}); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	for _, key := range []string{"/auth/doLogin", "/jobinfo/start", "/jobinfo/stop", "/jobinfo/trigger", "/jobinfo/delete", "/jobgroup/delete"} {
		if requests[key] != 1 {
			t.Errorf("expected exactly one %s request, got %d", key, requests[key])
		}
	}
	if requests["/login"] != 0 || requests["/jobinfo/remove"] != 0 {
		t.Fatal("legacy endpoint used for modern server")
	}
}

func TestModernRejectsLegacyPaginationResponse(t *testing.T) {
	// The envelope cannot be mistaken for a successful modern page.
	if _, err := normalizePage(map[string]any{"recordsFiltered": float64(1), "data": []any{}}, false); err == nil || !strings.Contains(err.Error(), "分页") {
		t.Fatal(err)
	}
}
