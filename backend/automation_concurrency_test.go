package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPolicyProcessWorker(t *testing.T) {
	dir := os.Getenv("XXL_TEST_STATE_DIR")
	if dir == "" {
		return
	}
	p := &plugin{dataDir: dir}
	for i := 0; i < 20; i++ {
		if e := p.lockedState(true, func(s *automationState) error {
			v := s.Policies["counter"]
			v.Revision++
			s.Policies["counter"] = v
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
}

func TestPolicyCrossProcessAtomicWrites(t *testing.T) {
	dir := t.TempDir()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	commands := []*exec.Cmd{}
	for i := 0; i < 3; i++ {
		cmd := exec.Command(executable, "-test.run=^TestPolicyProcessWorker$")
		cmd.Env = append(os.Environ(), "XXL_TEST_STATE_DIR="+dir)
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e)
		}
	}
	p := &plugin{dataDir: dir}
	if e := p.lockedState(false, func(s *automationState) error {
		if s.Policies["counter"].Revision != 60 {
			t.Errorf("lost updates: %+v", s.Policies)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}

func TestConcurrentSidecarsConsumeTokenOnce(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	grant(t, p, c, "trigger_job")
	args := map[string]any{"job_id": 7, "executor_id": 1, "parameter_mode": "saved"}
	args["preview_token"] = previewTool(t, p, c, "trigger_job", args)
	other := &plugin{dataDir: p.dataDir, sessions: map[string]*session{}}
	defer func() {
		for _, s := range other.sessions {
			s.close()
		}
	}()
	results := make(chan toolResult, 2)
	for _, target := range []*plugin{p, other} {
		go func(p *plugin) { results <- mcp(p, c, "trigger_job", args) }(target)
	}
	failures := 0
	for i := 0; i < 2; i++ {
		if (<-results).IsError {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("expected one refusal: %d", failures)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.writes) != 1 {
		t.Fatalf("replayed write: %v", state.writes)
	}
}

func TestRevocationWorksAfterReadOnlyOrLoginExpiry(t *testing.T) {
	p, c, _ := fixturePlugin(t, false)
	grant(t, p, c, "trigger_job")
	s := p.sessions[c.ID]
	s.mu.Lock()
	s.readOnly = true
	s.expired = true
	s.mu.Unlock()
	_, e := p.automationRPC("mcp/settings/set", jsonBytes(map[string]any{"connectionId": c.ID, "enabled": false, "revision": 2, "allowedTools": []string{"trigger_job"}}))
	if e != nil {
		t.Fatal(e)
	}
	if p.policyAllows(c.ID, "trigger_job") == nil {
		t.Fatal("revocation ignored")
	}
}

func TestExecutorRevisionMatchesUIEdit(t *testing.T) {
	p, c, _ := fixturePlugin(t, false)
	connectFixture(t, p, c)
	value, e := p.handle("xxljob/executorDetails", params{ConnectionID: c.ID, Form: map[string]any{"id": 1}})
	if e != nil {
		t.Fatal(e)
	}
	revision := value.(map[string]any)["revision"]
	form := map[string]any{"id": 1, "title": "Renamed", "expectedRevision": revision}
	preview, e := p.handle("xxljob/preview", params{ConnectionID: c.ID, Form: map[string]any{"operation": "xxljob/saveGroup", "arguments": form}})
	if e != nil {
		t.Fatal(e)
	}
	token := preview.(map[string]any)["previewToken"].(string)
	if _, e = p.handle("xxljob/saveGroup", params{ConnectionID: c.ID, Form: form, Confirmed: true, PreviewToken: token}); e != nil {
		t.Fatal(e)
	}
	form["expectedRevision"] = "stale"
	if _, e = p.handle("xxljob/preview", params{ConnectionID: c.ID, Form: map[string]any{"operation": "xxljob/saveGroup", "arguments": form}}); e == nil {
		t.Fatal("stale revision accepted")
	}
}

func TestTunnelSessionReuseAndRuntimeChange(t *testing.T) {
	p, c, state := fixturePlugin(t, false)
	endpoint := runtimeEndpoint{Host: "127.0.0.1"}
	// Fixture URL supplies a local transport; the logical target must remain internal.
	srvURL := c.Config.BaseURL
	var port string
	hostPort := strings.TrimPrefix(srvURL, "http://")
	_, port, _ = net.SplitHostPort(hostPort)
	endpoint.Port = int(number(port))
	c.Config.BaseURL = ""
	c.Config.ContextPath = "/"
	c.Host = "xxl.internal"
	c.Port = 8080
	c.TransportLayers = []transportLayer{{Type: "ssh"}}
	s, e := p.automationSession(params{Connection: c, Runtime: endpoint})
	if e != nil {
		t.Fatal(e)
	}
	reused, e := p.automationSession(params{Connection: c})
	if e != nil || reused != s {
		t.Fatal("session not reused", e)
	}
	if state.loginCount() != 1 {
		t.Fatal("relogged in")
	}
	changed := endpoint
	changed.Port++
	if _, e = p.automationSession(params{Connection: c, Runtime: changed}); e == nil {
		t.Fatal("stale tunnel reused")
	}
}

func TestDisconnectCancelsPendingRead(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	s, e := newSession(connection{Config: config{BaseURL: server.URL}}, runtimeEndpoint{})
	if e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() { result <- transportProbe(s) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("read not started")
	}
	s.close()
	select {
	case e := <-result:
		if e == nil {
			t.Fatal("cancelled read succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect did not cancel")
	}
	if s.ctx.Err() != context.Canceled {
		t.Fatal("session context not cancelled")
	}
}

func TestLogRangeAndOutputLimits(t *testing.T) {
	for _, f := range []map[string]any{{"from": "invalid"}, {"from": "2026-01-01 00:00:00", "to": "2026-03-01 00:00:00"}, {"from": "2026-03-01 00:00:00", "to": "2026-01-01 00:00:00"}} {
		if _, e := logRange(f); e == nil {
			t.Fatal("invalid range accepted", f)
		}
	}
	result := resultOf(map[string]any{"large": strings.Repeat("x", 200*1024)}, nil)
	if len(result.Content[0]["text"]) > 192*1024 || !strings.Contains(result.Content[0]["text"], "truncated") {
		t.Fatal("unbounded result")
	}
}

func TestDesktopBridgeWriteLossIsUnknownAndNotRetried(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DBX_APP_DATA_DIR", root)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(502) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	if e := os.WriteFile(filepath.Join(root, "mcp-bridge-port"), []byte(port), 0600); e != nil {
		t.Fatal(e)
	}
	p := &plugin{dataDir: t.TempDir()}
	r := p.forwardToDesktop("test", "trigger_job", map[string]any{"preview_token": "previously-approved-token"})
	if !r.IsError || !strings.Contains(r.Content[0]["text"], "结果未知") || calls != 1 {
		t.Fatal(r, calls)
	}
}

func TestDiagnosticSampleAndHostProviderBinding(t *testing.T) {
	p, c, _ := fixturePlugin(t, false)
	grant(t, p, c)
	r := resultObject(t, mcp(p, c, "get_job_diagnostics", map[string]any{"job_id": 7, "executor_id": 1}))
	d := r["diagnostics"].(map[string]any)
	if d["windowHours"] != float64(24) || len(d["recentFailures"].([]any)) != 1 || d["logBodiesIncluded"] != false {
		t.Fatal(d)
	}
	c.PluginProvider = "another-plugin.connection"
	if !mcp(p, c, "list_jobs", nil).IsError {
		t.Fatal("foreign host provider accepted")
	}
}
