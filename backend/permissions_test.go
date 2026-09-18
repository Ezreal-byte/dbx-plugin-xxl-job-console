package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFailedReconnectInvalidatesOldSession(t *testing.T) {
	p, srv := connected(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"code":200}`) }, false)
	_, e := call(p, "connection/connect", map[string]any{"connection": map[string]any{"id": "a", "username": "demo", "password": "", "external_config": map[string]any{"base_url": srv.URL}}})
	if e == nil {
		t.Fatal("invalid reconnect accepted")
	}
	if _, e = call(p, "xxljob/info", map[string]any{"connectionId": "a"}); e == nil {
		t.Fatal("old authenticated session survived failed reconnect")
	}
}

func TestAdministratorCanHaveNoExecutors(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobgroup/pageList" {
			t.Errorf("unexpected route %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}, false)
	rows, e := call(p, "xxljob/groups", map[string]any{"connectionId": "a"})
	if e != nil || len(rows.([]group)) != 0 {
		t.Fatal(rows, e)
	}
}

func TestNormalUserCannotAccessOtherExecutorsOrTasks(t *testing.T) {
	writes, groupReads, jobReads := 0, 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "operator-cookie", Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
		case "/":
			fmt.Fprint(w, `<a href="/jobinfo">Tasks</a>`)
		case "/jobinfo":
			fmt.Fprint(w, `<select id="jobGroup"><option value="1">Permitted</option></select>`)
		case "/joblog/getJobsByGroup":
			jobReads++
			if r.Form.Get("jobGroup") != "1" {
				t.Error("queried unpermitted group")
			}
			fmt.Fprint(w, `{"code":200,"content":[{"id":7,"jobGroup":1,"jobDesc":"Allowed","glueSource":"must-not-return-to-options"}]}`)
		case "/jobgroup/pageList":
			groupReads++
			fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
		case "/jobinfo/pageList":
			if r.Form.Get("jobGroup") != "1" {
				t.Error("queried all/unpermitted jobs")
			}
			fmt.Fprint(w, `{"recordsTotal":1,"recordsFiltered":1,"data":[{"id":7,"jobGroup":1}]}`)
		case "/jobinfo/trigger":
			writes++
			fmt.Fprint(w, `{"code":200}`)
		default:
			writes++
			fmt.Fprint(w, `{"code":200}`)
		}
	}))
	defer srv.Close()
	p := newPlugin()
	c := officialConnection(t, srv.URL, "operator", "test-password")
	if _, e := call(p, "connection/connect", map[string]any{"connection": c}); e != nil {
		t.Fatal(e)
	}
	defer call(p, "connection/disconnect", map[string]any{"connection": c})
	for _, method := range []string{"xxljob/jobs", "xxljob/logs", "xxljob/jobsByGroup"} {
		if _, e := call(p, method, map[string]any{"connectionId": "a", "form": map[string]any{"jobGroup": 2}}); e == nil {
			t.Fatalf("unpermitted %s accepted", method)
		}
	}
	if _, e := call(p, "xxljob/jobs", map[string]any{"connectionId": "a"}); e == nil {
		t.Fatal("ordinary query-all accepted")
	}
	if _, e := call(p, "xxljob/trigger", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 8}}); e == nil {
		t.Fatal("unpermitted task ID accepted")
	}
	if _, e := call(p, "xxljob/saveGroup", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"appname": "demo-executor", "title": "Demo", "addressType": 0}}); e == nil {
		t.Fatal("ordinary group management accepted")
	}
	groups, e := call(p, "xxljob/groups", map[string]any{"connectionId": "a"})
	if e != nil || len(groups.([]group)) != 1 {
		t.Fatal(groups, e)
	}
	options, e := call(p, "xxljob/jobsByGroup", map[string]any{"connectionId": "a", "form": map[string]any{"jobGroup": 1}})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range options.([]map[string]any) {
		if _, ok := v["glueSource"]; ok {
			t.Fatal("options exposed glue source")
		}
	}
	if writes != 0 || groupReads != 0 || jobReads < 1 {
		t.Fatalf("denied operation reached upstream %d %d %d", writes, groupReads, jobReads)
	}
	if _, e = call(p, "xxljob/trigger", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 7}}); e != nil || writes != 1 {
		t.Fatal(e, writes)
	}
}

func TestLoginCookiesNeverMixConnections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			r.ParseForm()
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: r.Form.Get("userName"), Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
			return
		}
		c, e := r.Cookie("XXL_JOB_LOGIN_IDENTITY")
		if e != nil {
			t.Error(e)
			return
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<a href="/jobgroup">Groups</a>`)
			return
		}
		r.ParseForm()
		if r.Form.Get("author") != c.Value {
			t.Error("cookies mixed")
		}
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}))
	defer srv.Close()
	p := newPlugin()
	defer func() {
		for _, id := range []string{"a", "b"} {
			call(p, "connection/disconnect", map[string]any{"connection": map[string]any{"id": id}})
		}
	}()
	for _, id := range []string{"a", "b"} {
		c := officialConnection(t, srv.URL, id, "test-password")
		c.ID = id
		if _, e := call(p, "connection/connect", map[string]any{"connection": c}); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, id := range []string{"a", "b"} {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if _, e := call(p, "xxljob/jobs", map[string]any{"connectionId": id, "form": map[string]any{"author": id}}); e != nil {
					t.Error(e)
				}
			}(id)
		}
	}
	wg.Wait()
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, s := range p.sessions {
		if s.password != "" {
			t.Fatal("plain password retained after login")
		}
	}
}

func TestLogReaderRejectsUnseenAndForgedExecutor(t *testing.T) {
	catCalls := 0
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/joblog/pageList" {
			fmt.Fprint(w, `{"recordsTotal":1,"recordsFiltered":1,"data":[{"id":9,"jobGroup":1,"jobId":7,"executorAddress":"http://node:9999/","triggerTime":1720000000000}]}`)
			return
		}
		catCalls++
		fmt.Fprint(w, `{"code":200}`)
	}, false)
	form := map[string]any{"executorAddress": "http://attacker.invalid:9999/", "triggerTime": 1720000000000, "logId": 9, "fromLineNum": 1}
	for _, queried := range []bool{false, true} {
		if queried {
			call(p, "xxljob/logs", map[string]any{"connectionId": "a"})
		}
		_, e := call(p, "xxljob/logContent", map[string]any{"connectionId": "a", "form": form})
		if e == nil || catCalls != 0 {
			t.Fatal("arbitrary executor contacted", e, catCalls)
		}
	}
}

func TestExpiredSessionInfoCannotReportConnected(t *testing.T) {
	var expired atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "demo", Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
			return
		}
		if expired.Load() {
			http.Redirect(w, r, "/toLogin", 302)
			return
		}
		fmt.Fprint(w, `<a href="/jobgroup">Groups</a>`)
	}))
	defer srv.Close()
	p := newPlugin()
	c := officialConnection(t, srv.URL, "demo", "test-only")
	if _, e := call(p, "connection/connect", map[string]any{"connection": c}); e != nil {
		t.Fatal(e)
	}
	defer call(p, "connection/disconnect", map[string]any{"connection": c})
	expired.Store(true)
	_, e := call(p, "xxljob/info", map[string]any{"connectionId": "a"})
	if e == nil || !strings.Contains(e.Message, "登录已失效") {
		t.Fatal(e)
	}
}

func TestRoleDowngradeImmediatelyRestrictsWrites(t *testing.T) {
	var downgraded atomic.Bool
	var writes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "demo", Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
		case "/":
			if downgraded.Load() {
				fmt.Fprint(w, `<a href="/jobinfo">Tasks</a>`)
			} else {
				fmt.Fprint(w, `<a href="/jobgroup">Groups</a>`)
			}
		case "/jobinfo":
			fmt.Fprint(w, `<select id="jobGroup"><option value="1">Permitted</option></select>`)
		case "/joblog/getJobsByGroup":
			fmt.Fprint(w, `{"code":200,"content":[{"id":7,"jobGroup":1,"jobDesc":"Allowed"}]}`)
		default:
			writes.Add(1)
			fmt.Fprint(w, `{"code":200}`)
		}
	}))
	defer srv.Close()
	p := newPlugin()
	c := officialConnection(t, srv.URL, "demo", "test-only")
	if _, e := call(p, "connection/connect", map[string]any{"connection": c}); e != nil {
		t.Fatal(e)
	}
	defer call(p, "connection/disconnect", map[string]any{"connection": c})
	downgraded.Store(true)
	for _, action := range []struct {
		method string
		form   map[string]any
	}{
		{"xxljob/removeGroup", map[string]any{"id": 2}},
		{"xxljob/trigger", map[string]any{"id": 8}},
	} {
		if _, e := call(p, action.method, map[string]any{"connectionId": "a", "confirmed": true, "form": action.form}); e == nil {
			t.Fatal("downgraded administrator retained permission")
		}
	}
	if writes.Load() != 0 {
		t.Fatal("denied write reached upstream")
	}
}
