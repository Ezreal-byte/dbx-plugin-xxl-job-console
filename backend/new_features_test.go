package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestUserPathsAcrossAdminVersions(t *testing.T) {
	for _, version := range []string{"2.3", "3.4"} {
		s := &session{adminVersion: version}
		form, err := buildForm("xxljob/users", map[string]any{"start": float64(25), "length": float64(25), "username": "demo", "role": float64(0)})
		if err != nil {
			t.Fatal(err)
		}
		path, adapted, err := s.adaptRequest("xxljob/users", "/user/pageList", form)
		if err != nil || path != "/user/pageList" {
			t.Fatalf("%s: %s %v", version, path, err)
		}
		if version == "3.4" && (adapted.Get("offset") != "25" || adapted.Get("pagesize") != "25" || adapted.Has("start")) {
			t.Fatalf("modern user pagination: %v", adapted)
		}
		if version == "2.3" && (adapted.Get("start") != "25" || adapted.Has("offset")) {
			t.Fatalf("legacy user pagination: %v", adapted)
		}
		path, adapted, err = s.adaptRequest("xxljob/removeUser", "/user/remove", url.Values{"id": {"7"}})
		if err != nil {
			t.Fatal(err)
		}
		if version == "3.4" && (path != "/user/delete" || adapted.Get("ids[]") != "7") {
			t.Fatalf("modern delete: %s %v", path, adapted)
		}
		if version == "2.3" && (path != "/user/remove" || adapted.Get("id") != "7") {
			t.Fatalf("legacy delete: %s %v", path, adapted)
		}
	}
}

func TestLegacyReportAndUserSecurity(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.URL.Path {
		case "/chartInfo":
			if r.Form.Get("startDate") != "2026-09-01 00:00:00" || r.Form.Get("endDate") != "2026-09-02 23:59:59" {
				t.Errorf("report range: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"content":{"triggerDayList":["2026-09-01"],"triggerDayCountRunningList":[3],"triggerDayCountSucList":[2],"triggerDayCountFailList":[1]}}`)
		case "/jobinfo/nextTriggerTime":
			if r.Form.Get("scheduleType") != "CRON" || r.Form.Get("scheduleConf") != "0 0/5 * * * ?" {
				t.Errorf("legacy cron form: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"content":["a","b","c","d","e"]}`)
		case "/user/pageList":
			fmt.Fprint(w, `{"recordsTotal":2,"recordsFiltered":2,"data":[{"id":1,"username":"demo","password":"secret","role":1},{"id":3,"username":"other","password":"hash","role":0}]}`)
		case "/user/remove":
			if r.Form.Get("id") != "3" {
				t.Errorf("wrong delete: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":200,"content":null}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}, false)
	report, err := call(p, "xxljob/report", map[string]any{"connectionId": "a", "form": map[string]any{"startDate": "2026-09-01 00:00:00", "endDate": "2026-09-02 23:59:59"}})
	if err != nil || report.(map[string]any)["triggerDayList"] == nil {
		t.Fatalf("report: %v %v", report, err)
	}
	times, err := call(p, "xxljob/nextTriggerTime", map[string]any{"connectionId": "a", "form": map[string]any{"scheduleType": "CRON", "scheduleConf": "0 0/5 * * * ?"}})
	if err != nil || len(times.([]any)) != 5 {
		t.Fatalf("legacy cron times: %v %v", times, err)
	}
	users, err := call(p, "xxljob/users", map[string]any{"connectionId": "a"})
	if err != nil {
		t.Fatal(err)
	}
	row := users.(map[string]any)["data"].([]any)[0].(map[string]any)
	if _, found := row["password"]; found {
		t.Fatal("password disclosed to UI")
	}
	if _, err := call(p, "xxljob/removeUser", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 1}}); err == nil {
		t.Fatal("current account deleted")
	}
	if _, err := call(p, "xxljob/removeUser", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 3}}); err != nil {
		t.Fatal(err)
	}
}

func TestReportDatesAndUserValidation(t *testing.T) {
	if _, err := buildForm("xxljob/report", map[string]any{"startDate": "2026-09-01 00:00:00", "endDate": "2026-09-14 23:59:59"}); err != nil {
		t.Fatal(err)
	}
	if _, err := buildForm("xxljob/report", map[string]any{"startDate": "2026-09-14 00:00:00", "endDate": "2026-09-01 23:59:59"}); err == nil {
		t.Fatal("accepted inverted range")
	}
	if _, err := buildForm("xxljob/report", map[string]any{"startDate": "2026-09-01", "endDate": "2026-09-14"}); err == nil {
		t.Fatal("accepted date without time")
	}
	if _, err := buildForm("xxljob/nextTriggerTime", map[string]any{"scheduleType": "CRON", "scheduleConf": "0 0/5 * * * ?"}); err != nil {
		t.Fatal(err)
	}
	if err := validateMutation("xxljob/saveUser", url.Values{"username": {"demo"}, "password": {"pass"}, "role": {"0"}, "permission": {"1,2"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateMutation("xxljob/saveUser", url.Values{"username": {"demo"}, "role": {"0"}}); err == nil {
		t.Fatal("accepted new user without password")
	}
}
