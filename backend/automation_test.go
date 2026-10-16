package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type automationFixture struct {
	mu        sync.Mutex
	jobs      map[int64]map[string]any
	writes    []string
	forms     []url.Values
	logins    int
	dropWrite bool
	modern    bool
}

func fixturePlugin(t *testing.T, modern bool) (*plugin, connection, *automationFixture) {
	t.Helper()
	state := &automationFixture{jobs: map[int64]map[string]any{}, modern: modern}
	job := defaultJob()
	job["id"] = float64(7)
	job["jobGroup"] = float64(1)
	job["jobDesc"] = "Demo"
	job["author"] = "tester"
	job["executorHandler"] = "demo"
	job["executorParam"] = "password=secret-value"
	job["triggerStatus"] = float64(0)
	state.jobs[7] = job
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		_ = r.ParseForm()
		reply := func(v any) {
			key := "content"
			if modern {
				key = "data"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, key: v})
		}
		page := func(rows []any, total int) {
			if modern {
				reply(map[string]any{"total": total, "data": rows})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"recordsTotal": total, "recordsFiltered": total, "data": rows})
			}
		}
		switch r.URL.Path {
		case "/login", "/auth/doLogin":
			state.logins++
			http.SetCookie(w, &http.Cookie{Name: "XXL_JOB_LOGIN_IDENTITY", Value: r.Form.Get("userName"), Path: "/"})
			reply(nil)
		case "/", "/dashboard":
			cookie, _ := r.Cookie("XXL_JOB_LOGIN_IDENTITY")
			if cookie != nil && cookie.Value == "operator" {
				fmt.Fprint(w, `<a href="/jobinfo">Jobs</a>`)
			} else {
				fmt.Fprint(w, `<a href="/jobgroup">Groups</a><span class="info-box-number">1</span><span class="info-box-number">2</span><span class="info-box-number">1</span>`)
			}
		case "/jobinfo":
			fmt.Fprint(w, `<select id="jobGroup"><option value="1">Permitted</option></select>`)
		case "/jobgroup/pageList":
			page([]any{map[string]any{"id": 1, "appname": "demo-executor", "title": "Demo executor", "addressType": 0, "addressList": "http://node:9999/"}}, 1)
		case "/joblog/getJobsByGroup":
			rows := []any{}
			for _, job := range state.jobs {
				rows = append(rows, job)
			}
			reply(rows)
		case "/jobinfo/pageList":
			rows := []any{}
			for _, job := range state.jobs {
				g := number(r.Form.Get("jobGroup"))
				if g <= 0 || g == number(job["jobGroup"]) {
					rows = append(rows, cloneMap(job))
				}
			}
			total := len(rows)
			offset := int(number(r.Form.Get("start")))
			limit := int(number(r.Form.Get("length")))
			if modern {
				offset = int(number(r.Form.Get("offset")))
				limit = int(number(r.Form.Get("pagesize")))
			}
			if offset > total {
				offset = total
			}
			end := offset + limit
			if end > total {
				end = total
			}
			page(rows[offset:end], total)
		case "/joblog/pageList":
			page([]any{map[string]any{"id": 11, "jobId": 7, "jobGroup": 1, "executorAddress": "http://node:9999/", "triggerTime": time.Now().Add(-time.Minute).UnixMilli(), "triggerCode": 200, "handleCode": 500, "executorParam": "private-param"}}, 1)
		case "/joblog/logDetailCat":
			from := number(r.Form.Get("fromLineNum"))
			text := ""
			for n := from; n <= 500; n++ {
				text += fmt.Sprintf("line %d token=private-log-token\n", n)
			}
			reply(map[string]any{"fromLineNum": from, "toLineNum": 500, "logContent": text, "end": true})
		case "/chartInfo":
			reply(map[string]any{"triggerDayList": []string{"2026-10-16"}})
		case "/jobinfo/nextTriggerTime":
			reply([]string{"2026-10-16 14:00:00"})
		default:
			state.writes = append(state.writes, r.URL.Path)
			state.forms = append(state.forms, r.Form)
			if state.dropWrite {
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}
			id := number(r.Form.Get("id"))
			if modern && id == 0 {
				id = number(r.Form.Get("ids[]"))
			}
			switch r.URL.Path {
			case "/jobinfo/start":
				state.jobs[id]["triggerStatus"] = float64(1)
			case "/jobinfo/stop":
				state.jobs[id]["triggerStatus"] = float64(0)
			case "/jobinfo/remove", "/jobinfo/delete":
				delete(state.jobs, id)
			case "/jobinfo/update":
				for k := range r.Form {
					state.jobs[id][k] = r.Form.Get(k)
				}
			case "/jobinfo/add", "/jobinfo/insert":
				copy := map[string]any{}
				for k := range r.Form {
					copy[k] = r.Form.Get(k)
				}
				copy["id"] = float64(8)
				copy["triggerStatus"] = float64(0)
				state.jobs[8] = copy
				reply("8")
				return
			}
			reply(nil)
		}
	}))
	t.Cleanup(server.Close)
	p := newPlugin()
	p.dataDir = t.TempDir()
	c := officialConnection(t, server.URL, "demo", "fixture-password")
	if modern {
		c.Config.AdminVersion = "3.4"
	}
	t.Cleanup(func() {
		p.mu.RLock()
		all := []*session{}
		for _, s := range p.sessions {
			all = append(all, s)
		}
		p.mu.RUnlock()
		for _, s := range all {
			s.close()
		}
	})
	return p, c, state
}
func connectFixture(t *testing.T, p *plugin, c connection) {
	t.Helper()
	if _, e := p.handle("connection/connect", params{Connection: c}); e != nil {
		t.Fatal(e)
	}
}
func grant(t *testing.T, p *plugin, c connection, names ...string) {
	t.Helper()
	connectFixture(t, p, c)
	v, e := p.settingsGet(c.ID)
	if e != nil {
		t.Fatal(e)
	}
	revision := v.(map[string]any)["policy"].(policy).Revision
	if _, e = p.automationRPC("mcp/settings/set", jsonBytes(map[string]any{"connectionId": c.ID, "enabled": true, "revision": revision})); e != nil {
		t.Fatal(e)
	}
	if names != nil {
		if _, e = p.automationRPC("mcp/settings/set", jsonBytes(map[string]any{"connectionId": c.ID, "enabled": true, "revision": revision + 1, "allowedTools": names})); e != nil {
			t.Fatal(e)
		}
	}
}
func mcp(p *plugin, c connection, name string, args map[string]any) toolResult {
	return p.mcpCall(jsonBytes(map[string]any{"tool": name, "arguments": args, "lifecycle": map[string]any{"connection": c, "connectionId": c.ID}}))
}
func resultObject(t *testing.T, result toolResult) map[string]any {
	t.Helper()
	if result.IsError {
		t.Fatal(result.Content)
	}
	var value map[string]any
	if e := json.Unmarshal([]byte(result.Content[0]["text"]), &value); e != nil {
		t.Fatal(e)
	}
	return value
}
func previewTool(t *testing.T, p *plugin, c connection, name string, args map[string]any) string {
	t.Helper()
	return resultObject(t, mcp(p, c, name, args))["previewToken"].(string)
}

func TestAutomationDiscoveryAndDefaultDeny(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	value, e := p.automationRPC("mcp/tools", jsonBytes(map[string]any{}))
	if e != nil || len(value.(map[string]any)["tools"].([]toolDefinition)) != 0 {
		t.Fatal(value, e)
	}
	if !mcp(p, c, "list_jobs", nil).IsError {
		t.Fatal("default allowed")
	}
	if state.logins != 0 {
		t.Fatal("discovery/denial dialed server")
	}
	grant(t, p, c)
	result := resultObject(t, mcp(p, c, "list_jobs", nil))
	if strings.Contains(string(jsonBytes(result)), "secret-value") {
		t.Fatal("job parameters leaked")
	}
	args := map[string]any{"host": "attacker"}
	if !mcp(p, c, "list_jobs", args).IsError {
		t.Fatal("endpoint override accepted")
	}
	if !mcp(p, c, "start_job", map[string]any{"job_id": 7, "executor_id": 1}).IsError {
		t.Fatal("query grant allowed write")
	}
}
func TestAutomationPolicyIsolationCorruptionAndRevision(t *testing.T) {
	p, c, _ := fixturePlugin(t, false)
	grant(t, p, c, "start_job")
	other := c
	other.ID = "other"
	if !mcp(p, other, "start_job", map[string]any{"job_id": 7, "executor_id": 1}).IsError {
		t.Fatal("policy crossed connections")
	}
	if _, e := p.automationRPC("mcp/settings/set", jsonBytes(map[string]any{"connectionId": c.ID, "enabled": true, "revision": 0})); e == nil {
		t.Fatal("stale policy revision accepted")
	}
	for _, enabled := range []bool{false, true} {
		v, _ := p.settingsGet(c.ID)
		rev := v.(map[string]any)["policy"].(policy).Revision
		if _, e := p.automationRPC("mcp/settings/set", jsonBytes(map[string]any{"connectionId": c.ID, "enabled": enabled, "revision": rev, "allowedTools": []string{"start_job"}})); e != nil {
			t.Fatal(e)
		}
	}
	if p.policyAllows(c.ID, "start_job") == nil || p.policyAllows(c.ID, "list_jobs") != nil {
		t.Fatal("re-enable did not reset to queries")
	}
	if e := os.WriteFile(filepath.Join(p.dataDir, "automation.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if !mcp(p, c, "list_jobs", nil).IsError {
		t.Fatal("corrupt settings allowed")
	}
	value, _ := p.settingsGet(c.ID)
	if value.(map[string]any)["warning"] == nil {
		t.Fatal("no corruption warning")
	}
}
func TestPreviewExecuteBothAdminProfiles(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			p, c, state := fixturePlugin(t, modern)
			grant(t, p, c, "start_job", "stop_job", "trigger_job", "update_job", "copy_job", "delete_job")
			args := map[string]any{"job_id": 7, "executor_id": 1}
			key := previewTool(t, p, c, "start_job", args)
			if len(state.writes) != 0 {
				t.Fatal("preview wrote")
			}
			args["preview_token"] = key
			resultObject(t, mcp(p, c, "start_job", args))
			if len(state.writes) != 1 {
				t.Fatal(state.writes)
			}
			if !mcp(p, c, "start_job", args).IsError || len(state.writes) != 1 {
				t.Fatal("token replay")
			}
			update := map[string]any{"job_id": 7, "executor_id": 1, "fields": map[string]any{"jobDesc": "Changed"}}
			update["preview_token"] = previewTool(t, p, c, "update_job", update)
			resultObject(t, mcp(p, c, "update_job", update))
			if state.forms[1].Get("executorParam") != "password=secret-value" {
				t.Fatal("patch wiped omitted parameter")
			}
			run := map[string]any{"job_id": 7, "executor_id": 1, "parameter_mode": "override", "executor_param": ""}
			run["preview_token"] = previewTool(t, p, c, "trigger_job", run)
			resultObject(t, mcp(p, c, "trigger_job", run))
			if !state.forms[2].Has("executorParam") || state.forms[2].Get("executorParam") != "" {
				t.Fatal("empty override lost")
			}
			copy := map[string]any{"job_id": 7, "executor_id": 1, "fields": map[string]any{}}
			copy["preview_token"] = previewTool(t, p, c, "copy_job", copy)
			resultObject(t, mcp(p, c, "copy_job", copy))
			if state.forms[3].Has("triggerStatus") || state.forms[3].Has("id") {
				t.Fatal("copy inherited identity/schedule")
			}
		})
	}
}
func TestPreviewBindingExpiryChangesAndReadonly(t *testing.T) {
	for _, scenario := range []string{"arguments", "connection", "expiry", "target", "readonly", "revoked", "credentials"} {
		t.Run(scenario, func(t *testing.T) {
			p, c, state := fixturePlugin(t, false)
			grant(t, p, c, "start_job")
			args := map[string]any{"job_id": 7, "executor_id": 1}
			key := previewTool(t, p, c, "start_job", args)
			switch scenario {
			case "arguments":
				args["job_id"] = 8
			case "connection":
				other := c
				other.ID = "other"
				grant(t, p, other, "start_job")
				c = other
			case "expiry":
				_ = p.lockedState(true, func(s *automationState) error {
					r := s.Previews[key]
					r.ExpiresAt = time.Now().Add(-time.Minute)
					s.Previews[key] = r
					return nil
				})
			case "target":
				state.mu.Lock()
				state.jobs[7]["jobDesc"] = "external change"
				state.mu.Unlock()
			case "readonly":
				c.ReadOnly = true
			case "credentials":
				c.Password = "changed"
			case "revoked":
				_ = p.lockedState(true, func(s *automationState) error {
					v := s.Policies[c.ID]
					v.Enabled = false
					s.Policies[c.ID] = v
					return nil
				})
			}
			args["preview_token"] = key
			if !mcp(p, c, "start_job", args).IsError || len(state.writes) != 0 {
				t.Fatal("invalid execution accepted")
			}
		})
	}
}
func TestPublicUIRequiresBoundPreview(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	connectFixture(t, p, c)
	form := map[string]any{"id": 7, "jobGroup": 1}
	if _, e := p.handle("xxljob/start", params{ConnectionID: c.ID, Form: form, Confirmed: true}); e == nil {
		t.Fatal("confirmed boolean bypassed preview")
	}
	v, e := p.handle("xxljob/preview", params{ConnectionID: c.ID, Form: map[string]any{"operation": "xxljob/start", "arguments": form}})
	if e != nil {
		t.Fatal(e)
	}
	if len(state.writes) != 0 {
		t.Fatal("UI preview wrote")
	}
	token := v.(map[string]any)["previewToken"].(string)
	if _, e = p.handle("xxljob/start", params{ConnectionID: c.ID, Form: form, Confirmed: true, PreviewToken: token}); e != nil || len(state.writes) != 1 {
		t.Fatal(e, state.writes)
	}
}
func TestColdMCPReadsLogsWithoutPriorList(t *testing.T) {
	p, c, _ := fixturePlugin(t, false)
	grant(t, p, c)
	p.mu.Lock()
	old := p.sessions[c.ID]
	delete(p.sessions, c.ID)
	p.mu.Unlock()
	old.close()
	args := map[string]any{"log_id": 11, "job_id": 7, "executor_id": 1, "limit": 25}
	v := resultObject(t, mcp(p, c, "read_log", args))
	if number(v["nextLine"]) != 26 || v["truncated"] != true || strings.Contains(textValue(v["logContent"]), "private-log-token") {
		t.Fatal(v)
	}
	args["job_id"] = 8
	if !mcp(p, c, "read_log", args).IsError {
		t.Fatal("forged log identity accepted")
	}
}
func TestExecutorReferencesAndTriggerAddresses(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	grant(t, p, c, "delete_executor", "trigger_job")
	if !mcp(p, c, "delete_executor", map[string]any{"executor_id": 1}).IsError {
		t.Fatal("in-use executor deletion allowed")
	}
	if !mcp(p, c, "trigger_job", map[string]any{"job_id": 7, "executor_id": 1, "parameter_mode": "saved", "address": "http://attacker/"}).IsError {
		t.Fatal("arbitrary executor address accepted")
	}
	if len(state.writes) != 0 {
		t.Fatal(state.writes)
	}
}
func TestAmbiguousWriteIsNotRetried(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	grant(t, p, c, "trigger_job")
	args := map[string]any{"job_id": 7, "executor_id": 1, "parameter_mode": "saved"}
	args["preview_token"] = previewTool(t, p, c, "trigger_job", args)
	state.mu.Lock()
	state.dropWrite = true
	state.mu.Unlock()
	result := mcp(p, c, "trigger_job", args)
	if !result.IsError || !strings.Contains(result.Content[0]["text"], "结果未知") || len(state.writes) != 1 {
		t.Fatal(result, state.writes)
	}
	if !mcp(p, c, "trigger_job", args).IsError || len(state.writes) != 1 {
		t.Fatal("ambiguous write replayed")
	}
}
func TestNonAdminCannotReadGlobalOrForeignData(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	c.Username = "operator"
	grant(t, p, c)
	for name, args := range map[string]map[string]any{"get_overview": {}, "list_jobs": {"executor_id": 2}, "get_job": {"executor_id": 2, "job_id": 7}, "get_log": {"executor_id": 2, "job_id": 7, "log_id": 11}} {
		if !mcp(p, c, name, args).IsError {
			t.Fatalf("unauthorized %s", name)
		}
	}
	resultObject(t, mcp(p, c, "list_jobs", map[string]any{"executor_id": 1}))
	if len(state.writes) != 0 {
		t.Fatal(state.writes)
	}
}
func TestTunnelDoesNotUseOriginalTarget(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	grant(t, p, c)
	p.mu.Lock()
	old := p.sessions[c.ID]
	delete(p.sessions, c.ID)
	p.mu.Unlock()
	old.close()
	c.TransportLayers = []transportLayer{{Type: "ssh"}}
	if !mcp(p, c, "list_jobs", nil).IsError {
		t.Fatal("missing tunnel accepted")
	}
	if state.logins != 1 {
		t.Fatal("tunnel fell back to direct login", state.logins)
	}
}
