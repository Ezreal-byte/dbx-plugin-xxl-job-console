package main

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"
)

// Opt-in, disposable localhost Admin only. Creates its own executor and jobs;
// never point this test at a daily-use Admin. See VERIFICATION_2.2.0.md.
func TestLiveAdmin22(t *testing.T) {
	endpoint := os.Getenv("XXLJOB22_TEST_URL")
	if endpoint == "" {
		t.Skip("set XXLJOB22_TEST_URL and XXLJOB22_TEST_PASSWORD for isolated integration")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() != "127.0.0.1" || os.Getenv("XXLJOB22_TEST_PASSWORD") == "" {
		t.Fatal("requires an explicit localhost test URL and password")
	}
	p := newPlugin()
	p.dataDir = t.TempDir()
	c := officialConnection(t, endpoint, "admin", os.Getenv("XXLJOB22_TEST_PASSWORD"))
	c.ID, c.Name, c.Config.AdminVersion = "issue6-live22", "Isolated 2.2.0 verification", "2.2"
	t.Cleanup(func() {
		for _, s := range p.sessions {
			s.close()
		}
	})
	if !mcp(p, c, "list_jobs", nil).IsError {
		t.Fatal("default permission unexpectedly enabled")
	}
	names := []string{}
	for _, tool := range toolCatalog() {
		names = append(names, tool.Name)
	}
	grant(t, p, c, names...)
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		return resultObject(t, mcp(p, c, name, args))
	}
	write := func(name string, args map[string]any) map[string]any {
		t.Helper()
		preview := call(name, args)
		if preview["executed"] != false || preview["previewToken"] == "" {
			t.Fatal("missing write preview", name)
		}
		args["preview_token"] = preview["previewToken"]
		return call(name, args)
	}
	uiWrite := func(method string, form map[string]any) {
		t.Helper()
		value, e := p.handle("xxljob/preview", params{ConnectionID: c.ID, Form: map[string]any{"operation": method, "arguments": form}})
		if e != nil {
			t.Fatal(e)
		}
		_, e = p.handle(method, params{ConnectionID: c.ID, Form: form, Confirmed: true, PreviewToken: value.(map[string]any)["previewToken"].(string)})
		if e != nil {
			t.Fatal(e)
		}
	}
	call("connection_info", nil)
	call("get_overview", nil)
	call("preview_schedule", map[string]any{"cron": "0 0 0 1 1 ? 2099"})
	appname := fmt.Sprintf("dbx-issue6-%d", time.Now().Unix())
	uiWrite("xxljob/saveGroup", map[string]any{"appname": appname, "title": "DBX22 test", "addressType": 0, "addressList": ""})
	value, err := p.handle("xxljob/groups", params{ConnectionID: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	var gid int64
	for _, g := range value.([]group) {
		if g.Appname == appname {
			gid = int64(g.ID)
		}
	}
	if gid == 0 {
		t.Fatal("new executor not found")
	}
	jobs := []int64{}
	t.Cleanup(func() {
		for _, id := range jobs {
			args := map[string]any{"job_id": id, "executor_id": gid}
			preview := mcp(p, c, "delete_job", args)
			if preview.IsError {
				t.Error("cleanup preview failed", id, preview.Content)
				continue
			}
			args["preview_token"] = resultObject(t, preview)["previewToken"]
			if result := mcp(p, c, "delete_job", args); result.IsError {
				t.Error("cleanup delete failed", id, result.Content)
			}
		}
		write("delete_executor", map[string]any{"executor_id": gid})
	})
	created := write("create_job", map[string]any{"fields": map[string]any{"jobGroup": gid, "jobDesc": "Issue6 isolated job", "author": "dbx-test", "executorHandler": "demoJobHandler", "executorParam": "preserve-this", "scheduleConf": "0 0 0 1 1 ? 2099"}})
	id := number(created["result"])
	if id <= 0 {
		t.Fatal("create did not return an ID", created)
	}
	jobs = append(jobs, id)
	readJob := func(id int64) map[string]any {
		t.Helper()
		v, e := p.handle("xxljob/jobDetails", params{ConnectionID: c.ID, Form: map[string]any{"id": id, "jobGroup": gid}})
		if e != nil {
			t.Fatal(e)
		}
		return v.(map[string]any)["job"].(map[string]any)
	}
	job := readJob(id)
	if job["scheduleConf"] != "0 0 0 1 1 ? 2099" || number(job["triggerStatus"]) != 0 {
		t.Fatal("create schedule mismatch", job)
	}
	uiWrite("xxljob/saveJob", map[string]any{"id": id, "jobGroup": gid, "jobDesc": "Updated through UI", "scheduleConf": "0 0 1 1 1 ? 2099"})
	job = readJob(id)
	if job["scheduleConf"] != "0 0 1 1 1 ? 2099" || job["executorParam"] != "preserve-this" {
		t.Fatal("update cleared configuration", job)
	}
	for _, name := range []string{"start_job", "stop_job"} {
		write(name, map[string]any{"job_id": id, "executor_id": gid})
		want := int64(0)
		if name == "start_job" {
			want = 1
		}
		if number(readJob(id)["triggerStatus"]) != want {
			t.Fatal("schedule status mismatch", name)
		}
	}
	copy := write("copy_job", map[string]any{"job_id": id, "executor_id": gid, "fields": map[string]any{"jobDesc": "Issue6 copy"}})
	copyID := number(copy["result"])
	if copyID <= 0 {
		t.Fatal("copy did not return an ID")
	}
	jobs = append(jobs, copyID)
	if number(readJob(copyID)["triggerStatus"]) != 0 {
		t.Fatal("copy enabled scheduling")
	}
	write("trigger_job", map[string]any{"job_id": id, "executor_id": gid, "parameter_mode": "saved"})
	// There is intentionally no executor node. Verify Admin records dispatch failure,
	// not a successful business execution, after accepting the trigger request.
	deadline := time.Now().Add(10 * time.Second)
	for {
		v, e := p.handle("xxljob/logs", params{ConnectionID: c.ID, Form: map[string]any{"jobId": id, "jobGroup": gid}})
		if e != nil {
			t.Fatal(e)
		}
		rows := v.(map[string]any)["data"].([]any)
		if len(rows) > 0 && number(rows[0].(map[string]any)["triggerCode"]) == 500 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no completed dispatch failure record")
		}
		time.Sleep(200 * time.Millisecond)
	}
	call("get_job", map[string]any{"job_id": id, "executor_id": gid})
	call("list_logs", map[string]any{"job_id": id, "executor_id": gid})
	t.Log("real Admin 2.2.0: queries, Cron, UI executor create/job edit, MCP create/copy/start/stop/trigger, scoped log lookup passed; cleanup deletes test resources")
}
