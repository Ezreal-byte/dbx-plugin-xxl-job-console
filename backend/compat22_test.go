package main

import (
	"strings"
	"testing"
)

func TestAdmin22QueriesAndScheduleConstraints(t *testing.T) {
	p, c, state := fixturePluginVersion(t, "2.2")
	if !mcp(p, c, "list_jobs", nil).IsError || state.loginCount() != 0 {
		t.Fatal("default deny must not log in")
	}
	grant(t, p, c, "list_jobs", "get_job", "preview_schedule", "get_overview", "read_log", "update_job", "create_job")
	for _, query := range []struct {
		name string
		args map[string]any
	}{
		{"list_jobs", nil},
		{"get_job", map[string]any{"job_id": 7, "executor_id": 1}},
		{"preview_schedule", map[string]any{"cron": "0 0/5 * * * ?"}},
		{"get_overview", nil},
		{"read_log", map[string]any{"log_id": 11, "job_id": 7, "executor_id": 1, "limit": 2}},
	} {
		r := mcp(p, c, query.name, query.args)
		if r.IsError {
			t.Fatalf("%s: %v", query.name, r.Content)
		}
		if query.name == "get_job" && (!strings.Contains(r.Content[0]["text"], `"scheduleConf":"0 0/5 * * * ?"`) || strings.Contains(r.Content[0]["text"], "jobCron")) {
			t.Fatal("2.2 job model was not normalized", r.Content)
		}
	}
	if _, err := p.handle("xxljob/jobsByGroup", params{ConnectionID: c.ID, Form: map[string]any{"jobGroup": 1}}); err != nil {
		t.Fatal(err)
	}
	for _, fields := range []map[string]any{
		{"scheduleType": "NONE"}, {"scheduleType": "FIX_RATE", "scheduleConf": "30"}, {"misfireStrategy": "FIRE_ONCE_NOW"},
	} {
		args := map[string]any{"job_id": 7, "executor_id": 1, "fields": fields}
		if !mcp(p, c, "update_job", args).IsError {
			t.Fatal("unsupported 2.2 scheduling accepted by MCP", fields)
		}
		form := cloneMap(fields)
		form["id"], form["jobGroup"] = 7, 1
		if _, err := p.handle("xxljob/preview", params{ConnectionID: c.ID, Form: map[string]any{"operation": "xxljob/saveJob", "arguments": form}}); err == nil {
			t.Fatal("unsupported 2.2 scheduling accepted by UI", fields)
		}
	}
	if len(state.recordedWrites()) != 0 {
		t.Fatal("queries or rejected previews wrote")
	}
	args := map[string]any{"fields": map[string]any{"jobGroup": 1, "jobDesc": "New", "author": "tester", "executorHandler": "demo"}}
	args["preview_token"] = previewTool(t, p, c, "create_job", args)
	resultObject(t, mcp(p, c, "create_job", args))
	form := state.recordedForm(0)
	if form.Get("jobCron") != "0 0/5 * * * ?" || form.Has("triggerStatus") || state.recordedWrites()[0] != "/jobinfo/add" {
		t.Fatal("2.2 create contract", form)
	}
}

func TestAdmin22MalformedJobFailsClosed(t *testing.T) {
	for _, row := range []any{nil, "invalid", map[string]any{}, map[string]any{"jobCron": 123}, map[string]any{"jobCron": ""}} {
		if normalize22Job(row) == nil {
			t.Fatal("missing Cron must not silently become a default schedule")
		}
	}
}
