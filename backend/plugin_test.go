package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

func call(p *plugin, method string, v any) (any, *sdk.PluginError) {
	b, _ := json.Marshal(v)
	return p.Handle(sdk.RequestContext{}, method, b, nil)
}
func connected(t *testing.T, handler http.HandlerFunc, readonly bool) (*plugin, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: "test-cookie", Path: "/"})
			fmt.Fprint(w, `{"code":200}`)
			return
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<html><a href="/jobgroup">Groups</a></html>`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	p := newPlugin()
	_, e := call(p, "connection/connect", map[string]any{"connection": map[string]any{"id": "a", "name": "test", "username": "demo", "password": "test-only", "read_only": readonly, "external_config": map[string]any{"base_url": srv.URL}}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { call(p, "connection/disconnect", map[string]any{"connection": map[string]any{"id": "a"}}) })
	return p, srv
}
func TestMutationsRequireConfirmationAndRespectReadonly(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		t.Run(fmt.Sprint(readonly), func(t *testing.T) {
			calls := 0
			p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{"code":200}`) }, readonly)
			methods := []string{"xxljob/start", "xxljob/stop", "xxljob/trigger", "xxljob/removeJob", "xxljob/saveJob", "xxljob/saveGroup", "xxljob/removeGroup"}
			for _, method := range methods {
				_, e := call(p, method, map[string]any{"connectionId": "a", "form": map[string]any{"id": 7}})
				if e == nil {
					t.Fatalf("%s unconfirmed accepted", method)
				}
			}
			if readonly {
				_, e := call(p, "xxljob/trigger", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 7, "executorParam": "x"}})
				if e == nil {
					t.Fatal("readonly trigger accepted")
				}
			}
			if calls != 0 {
				t.Fatalf("blocked mutation reached service %d", calls)
			}
		})
	}
}
func TestTriggerKeepsBlankOverrideAndLegacyForm(t *testing.T) {
	calls := 0
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		r.ParseForm()
		if r.URL.Path != "/jobinfo/trigger" || r.Method != "POST" || r.Form.Get("id") != "7" {
			t.Errorf("bad trigger %v", r)
		}
		if _, ok := r.Form["executorParam"]; !ok || r.Form.Get("executorParam") != "" {
			t.Errorf("blank override lost %v", r.Form)
		}
		if !r.Form.Has("addressList") || r.Form.Get("addressList") != "" {
			t.Errorf("blank address list missing %v", r.Form)
		}
		fmt.Fprint(w, `{"code":200,"content":null}`)
	}, false)
	_, e := call(p, "xxljob/trigger", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 7, "executorParam": ""}})
	if e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestTriggerPassesConfiguredParamToLegacyAdmin(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.URL.Path != "/jobinfo/trigger" || r.Form.Get("executorParam") != "key=配置值" {
			t.Errorf("configured execution parameter lost: %v", r.Form)
		}
		fmt.Fprint(w, `{"code":200,"content":null}`)
	}, false)
	if _, e := call(p, "xxljob/trigger", map[string]any{"connectionId": "a", "confirmed": true, "form": map[string]any{"id": 7, "executorParam": "key=配置值"}}); e != nil {
		t.Fatal(e)
	}
}
func TestReadMethodsUseOfficialStatusAndAuthor(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("author") != "demo" || r.Form.Get("triggerStatus") != "1" || r.Form.Get("length") != "25" {
			t.Errorf("bad filters %v", r.Form)
		}
		fmt.Fprint(w, `{"recordsTotal":1,"recordsFiltered":1,"data":[{"id":9}]}`)
	}, false)
	result, e := call(p, "xxljob/jobs", map[string]any{"connectionId": "a", "form": map[string]any{"author": "demo", "triggerStatus": 1}})
	if e != nil {
		t.Fatal(e)
	}
	if result.(map[string]any)["recordsFiltered"] != float64(1) {
		t.Fatal(result)
	}
}
func TestDisconnectAndUnknownMethodsCannotReachService(t *testing.T) {
	calls := 0
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) { calls++ }, false)
	_, e := call(p, "xxljob/raw", map[string]any{"connectionId": "a", "path": "/jobapi/v1/7/remove"})
	if e == nil {
		t.Fatal("arbitrary method allowed")
	}
	call(p, "connection/disconnect", map[string]any{"connection": map[string]any{"id": "a"}})
	_, e = call(p, "xxljob/info", map[string]any{"connectionId": "a"})
	if e == nil {
		t.Fatal("disconnected session usable")
	}
	if calls != 0 {
		t.Fatal(calls)
	}
}
func TestGroupHTMLParserReadsPermittedOptionsAndEntities(t *testing.T) {
	groups, e := parseGroups([]byte(`<select id="jobGroup"><option value="3">A &amp; B</option></select>`))
	if e != nil {
		t.Fatal(e)
	}
	if len(groups) != 1 || groups[0].Title != "A & B" || groups[0].ID != 3 {
		t.Fatalf("bad groups %+v", groups)
	}
	for _, html := range []string{`<html>login</html>`, `<select id="jobGroup"><option value="x">invalid</option></select>`} {
		if _, e = parseGroups([]byte(html)); e == nil {
			t.Fatal("malformed groups accepted")
		}
	}
	groups, e = parseGroups([]byte(`<select id="jobGroup"></select>`))
	if e != nil || len(groups) != 0 {
		t.Fatal(groups, e)
	}
}
func TestGroupWritesUseOfficialAppnameAndRejectInvalid(t *testing.T) {
	calls := 0
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		r.ParseForm()
		if r.URL.Path != "/jobgroup/save" || r.Form.Get("appname") != "demo-executor" || r.Form.Has("taskLimit") || r.Form.Has("order") || r.Form.Get("addressType") != "1" {
			t.Errorf("bad group %s %v", r.URL.Path, r.Form)
		}
		fmt.Fprint(w, `{"code":200}`)
	}, false)
	form := map[string]any{"appname": "demo-executor", "title": "Demo", "addressType": 1, "addressList": "http://node:9999/"}
	_, e := call(p, "xxljob/saveGroup", map[string]any{"connectionId": "a", "confirmed": true, "form": form})
	if e != nil {
		t.Fatal(e)
	}
	form["addressType"] = 2
	_, e = call(p, "xxljob/saveGroup", map[string]any{"connectionId": "a", "confirmed": true, "form": form})
	if e == nil {
		t.Fatal("invalid addressType accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestPaginationRejectsFractionalAndOversizedValues(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid pagination reached service") }, false)
	for _, v := range []any{-1, 1.5, "abc"} {
		_, e := call(p, "xxljob/logs", map[string]any{"connectionId": "a", "form": map[string]any{"start": v}})
		if e == nil {
			t.Fatal("bad start accepted")
		}
	}
	_, e := call(p, "xxljob/logs", map[string]any{"connectionId": "a", "form": map[string]any{"length": 101}})
	if e == nil {
		t.Fatal("oversized page accepted")
	}
}
func TestLogContentPreservesEndContract(t *testing.T) {
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/joblog/pageList" {
			fmt.Fprint(w, `{"recordsTotal":1,"recordsFiltered":1,"data":[{"id":9,"jobGroup":1,"jobId":7,"executorAddress":"http://node:9999/","triggerTime":1720000000000}]}`)
			return
		}
		r.ParseForm()
		if r.URL.Path != "/joblog/logDetailCat" || r.Form.Get("fromLineNum") != "1" || r.Form.Get("triggerTime") != "1720000000000" {
			t.Errorf("bad log %v", r.Form)
		}
		fmt.Fprint(w, `{"code":200,"content":{"fromLineNum":1,"toLineNum":2,"logContent":"one\ntwo","end":true}}`)
	}, false)
	if _, e := call(p, "xxljob/logs", map[string]any{"connectionId": "a"}); e != nil {
		t.Fatal(e)
	}
	v, e := call(p, "xxljob/logContent", map[string]any{"connectionId": "a", "form": map[string]any{"executorAddress": "http://node:9999/", "triggerTime": 1720000000000, "logId": 9, "fromLineNum": 1}})
	if e != nil {
		t.Fatal(e)
	}
	if v.(map[string]any)["end"] != true {
		t.Fatal(v)
	}
}
func TestJobValidationKeepsAllEditableLegacyFields(t *testing.T) {
	calls := 0
	p, _ := connected(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		r.ParseForm()
		if r.URL.Path != "/jobinfo/add" || r.Form.Get("scheduleType") != "CRON" || r.Form.Get("scheduleConf") != "0 0/5 * * * ?" || r.Form.Get("misfireStrategy") != "DO_NOTHING" || r.Form.Has("jobCron") || r.Form.Get("childJobId") != "8,9" || r.Form.Get("executorFailRetryCount") != "2" {
			t.Errorf("bad job %v", r.Form)
		}
		fmt.Fprint(w, `{"code":200,"content":"7"}`)
	}, false)
	form := map[string]any{"jobGroup": 3, "scheduleType": "CRON", "scheduleConf": "0 0/5 * * * ?", "misfireStrategy": "DO_NOTHING", "jobDesc": "Demo", "author": "demo", "executorRouteStrategy": "FIRST", "executorHandler": "demoHandler", "executorParam": "", "executorBlockStrategy": "SERIAL_EXECUTION", "executorTimeout": 0, "executorFailRetryCount": 2, "glueType": "BEAN", "childJobId": "8,9"}
	_, e := call(p, "xxljob/saveJob", map[string]any{"connectionId": "a", "confirmed": true, "form": form})
	if e != nil {
		t.Fatal(e)
	}
	form["glueType"] = "GLUE_SHELL"
	_, e = call(p, "xxljob/saveJob", map[string]any{"connectionId": "a", "confirmed": true, "form": form})
	if e == nil {
		t.Fatal("script creation accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestReturnTBusinessErrorIsVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"code":500,"msg":"超出任务上限"}`) }))
	defer srv.Close()
	s, _ := newSession(connection{Config: config{BaseURL: srv.URL}}, runtimeEndpoint{})
	defer s.close()
	_, e := s.request("/jobinfo/add", url.Values{}, true)
	if e == nil || !strings.Contains(e.Error(), "超出任务上限") {
		t.Fatal(e)
	}
}
