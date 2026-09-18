package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func officialConnection(t *testing.T, endpoint string, username, password string) connection {
	t.Helper()
	var c connection
	data, _ := json.Marshal(map[string]any{"id": "a", "name": "demo", "username": username, "password": password, "external_config": map[string]any{"base_url": endpoint}})
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOfficialLoginAndMemoryCookie(t *testing.T) {
	login, protected := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			login++
			r.ParseForm()
			if r.Method != "POST" || r.Form.Get("userName") != "demo" || r.Form.Get("password") != "test-only-password" || r.Form.Has("ifRemember") {
				t.Errorf("wrong login contract")
			}
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "fixture-cookie", Path: "/"})
			fmt.Fprint(w, `{"code":200,"content":null}`)
			return
		}
		protected++
		if c, e := r.Cookie("XXL_JOB_LOGIN_IDENTITY"); e != nil || c.Value != "fixture-cookie" {
			t.Error("authenticated cookie missing")
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<html><a href="/jobgroup">Groups</a></html>`)
			return
		}
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}))
	defer srv.Close()
	s, e := newSession(officialConnection(t, srv.URL, "demo", "test-only-password"), runtimeEndpoint{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if e = s.probe(); e != nil {
		t.Fatal(e)
	}
	if login != 1 || protected < 1 {
		t.Fatalf("login=%d protected=%d", login, protected)
	}
}

func TestLoginRejectsMissingCookieAndSanitizesErrors(t *testing.T) {
	for _, body := range []string{`{"code":200}`, `{"code":500,"msg":"test-only-password fixture-cookie"}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		s, e := newSession(officialConnection(t, srv.URL, "demo", "test-only-password"), runtimeEndpoint{})
		if e != nil {
			t.Fatal(e)
		}
		e = s.probe()
		s.close()
		srv.Close()
		if e == nil || strings.Contains(e.Error(), "test-only-password") || strings.Contains(e.Error(), "fixture-cookie") {
			t.Fatalf("bad safe login error %v", e)
		}
	}
}

func TestLoginNeverFollowsRedirect(t *testing.T) {
	leaked := 0
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++; w.WriteHeader(200) }))
	defer evil.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	s, e := newSession(officialConnection(t, srv.URL, "demo", "test-only-password"), runtimeEndpoint{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if e = s.probe(); e == nil || leaked != 0 {
		t.Fatalf("redirect accepted/leaked %v %d", e, leaked)
	}
}

func TestOfficialExpiredWriteIsNotRetried(t *testing.T) {
	calls, login := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			login++
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "fixture-cookie", Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
			return
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<html><a href="/jobgroup">Groups</a></html>`)
			return
		}
		calls++
		http.Redirect(w, r, "/toLogin", 302)
	}))
	defer srv.Close()
	s, e := newSession(officialConnection(t, srv.URL, "demo", "test-only-password"), runtimeEndpoint{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if e = s.probe(); e != nil {
		t.Fatal(e)
	}
	_, e = s.request("/jobinfo/trigger", url.Values{"id": {"7"}}, true)
	if e == nil || !strings.Contains(e.Error(), "结果未知") || calls != 1 || login != 1 {
		t.Fatalf("expired write unsafe %v calls=%d login=%d", e, calls, login)
	}
	_, e = s.request("/jobinfo/trigger", url.Values{"id": {"7"}}, true)
	if e == nil || calls != 1 {
		t.Fatalf("invalid session reused %v %d", e, calls)
	}
}
