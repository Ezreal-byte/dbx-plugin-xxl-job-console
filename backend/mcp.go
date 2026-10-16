package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type toolResult struct {
	Content []map[string]string `json:"content"`
	IsError bool                `json:"isError"`
}

var secretPattern = regexp.MustCompile(`(?i)((?:password|passwd|token|secret|authorization|api[_-]?key)\s*[=:]\s*)([^\s,;]+)`)

func sanitize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range v {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "password") || strings.Contains(lower, "cookie") || strings.Contains(lower, "secret") || lower == "gluesource" || lower == "executorparam" || lower == "alarmemail" {
				continue
			}
			out[k] = sanitize(x)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = sanitize(x)
		}
		return out
	case string:
		return secretPattern.ReplaceAllString(v, "${1}[REDACTED]")
	default:
		return v
	}
}
func resultOf(value any, err error) toolResult {
	var text string
	if err != nil {
		text = secretPattern.ReplaceAllString(err.Error(), "${1}[REDACTED]")
	} else {
		var normalized any
		_ = json.Unmarshal(jsonBytes(value), &normalized)
		text = string(jsonBytes(sanitize(normalized)))
	}
	if len(text) > 192*1024 {
		text = `{"truncated":true,"message":"结果超过192KiB，请缩小查询范围或 limit"}`
	}
	return toolResult{[]map[string]string{{"type": "text", "text": text}}, err != nil}
}

var errTunnelSession = errors.New("没有有效 DBX 隧道会话，请先打开此连接的 XXL-JOB 工作台；不会回退直连")

func hasTunnel(c connection) bool {
	for _, layer := range c.TransportLayers {
		if layer.Enabled == nil || *layer.Enabled {
			return true
		}
	}
	return false
}
func validRuntime(c connection, r runtimeEndpoint) bool {
	ip := net.ParseIP(strings.Trim(r.Host, "[]"))
	return ip != nil && ip.IsLoopback() && r.Port > 0 && r.Port <= 65535 && (r.Host != c.Host || r.Port != c.Port)
}
func (p *plugin) automationSession(v params) (*session, error) {
	c := v.Connection
	if c.ID == "" || (v.ConnectionID != "" && v.ConnectionID != c.ID) || (c.PluginID != "" && c.PluginID != pluginID) || (c.ProviderID != "" && c.ProviderID != pluginID+".connection") || (c.PluginProvider != "" && c.PluginProvider != pluginID+".connection") {
		return nil, errors.New("必须使用宿主绑定到 XXL-JOB 插件的连接")
	}
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	p.mu.RLock()
	s := p.sessions[c.ID]
	p.mu.RUnlock()
	if s != nil {
		if s.identity != connectionIdentity(c) || (hasTunnel(c) && validRuntime(c, v.Runtime) && v.Runtime != s.runtime) {
			s.close()
			p.mu.Lock()
			delete(p.sessions, c.ID)
			p.mu.Unlock()
			return nil, errors.New("连接配置或凭据已变化，请重新连接")
		}
		s.mu.Lock()
		s.readOnly = c.ReadOnly || c.Config.ReadOnly
		s.production = c.IsProduction || c.Config.IsProduction
		s.mu.Unlock()
		if !hasTunnel(c) || validRuntime(c, s.runtime) {
			return s, nil
		}
	}
	if hasTunnel(c) && !validRuntime(c, v.Runtime) {
		return nil, errTunnelSession
	}
	candidate, e := newSession(c, v.Runtime)
	if e != nil {
		return nil, e
	}
	if e = candidate.probe(); e != nil {
		candidate.close()
		return nil, e
	}
	p.mu.Lock()
	p.sessions[c.ID] = candidate
	p.mu.Unlock()
	return candidate, nil
}
func (p *plugin) mcpCall(raw json.RawMessage) toolResult {
	var in struct {
		Tool      string         `json:"tool"`
		Arguments map[string]any `json:"arguments"`
		Lifecycle params         `json:"lifecycle"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return resultOf(nil, errors.New("无效工具请求"))
	}
	if in.Arguments == nil {
		in.Arguments = map[string]any{}
	}
	forwarded := false
	if v, ok := in.Arguments["_dbxXxlForwarded"]; ok {
		if v != true {
			return resultOf(nil, errors.New("无效转发标记"))
		}
		forwarded = true
		delete(in.Arguments, "_dbxXxlForwarded")
	}
	t, ok := findTool(in.Tool)
	if !ok {
		return resultOf(nil, errors.New("未知 XXL-JOB 工具"))
	}
	if e := validateValue(t.InputSchema, in.Arguments, "arguments"); e != nil {
		return resultOf(nil, e)
	}
	id := in.Lifecycle.Connection.ID
	started := time.Now()
	outcome := "rejected"
	auditedByMutation := false
	defer func() {
		if !auditedByMutation {
			resource := textValue(in.Arguments["job_id"])
			if resource == "" {
				resource = textValue(in.Arguments["executor_id"])
			}
			p.audit(id, t.Name, resource, outcome, started)
		}
	}()
	if e := p.policyAllows(id, t.Name); e != nil {
		return resultOf(nil, e)
	}
	if t.Group != "query" && (in.Lifecycle.Connection.ReadOnly || in.Lifecycle.Connection.Config.ReadOnly) {
		return resultOf(nil, errors.New("只读连接禁止修改"))
	}
	s, e := p.automationSession(in.Lifecycle)
	if errors.Is(e, errTunnelSession) && !forwarded {
		result := p.forwardToDesktop(id, t.Name, in.Arguments)
		if !result.IsError {
			outcome = "forwarded"
		} else if len(result.Content) > 0 && strings.Contains(result.Content[0]["text"], "结果未知") {
			outcome = "unknown"
		}
		return result
	}
	if e != nil {
		return resultOf(nil, e)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	s.opContext = ctx
	defer func() { s.opContext = nil }()
	if e = s.active(); e != nil {
		return resultOf(nil, e)
	}
	if e = p.policyAllows(id, t.Name); e != nil {
		return resultOf(nil, e)
	}
	var value any
	if t.Group == "query" {
		value, e = p.queryTool(s, t.Name, in.Arguments)
	} else {
		auditedByMutation = true
		token := textValue(in.Arguments["preview_token"])
		delete(in.Arguments, "preview_token")
		value, e = p.mutation(s, t.Name, in.Arguments, token, true)
	}
	if e == nil {
		outcome = "success"
	}
	return resultOf(value, e)
}
func (p *plugin) forwardToDesktop(id, tool string, args map[string]any) toolResult {
	// A broken bridge response may follow a completed write; never invite a retry.
	bridgeFailure := func(message string) toolResult {
		if definition, ok := findTool(tool); ok && definition.Group != "query" && textValue(args["preview_token"]) != "" {
			return resultOf(nil, uncertain(true, message))
		}
		return resultOf(nil, errors.New(message))
	}
	root := os.Getenv("DBX_APP_DATA_DIR")
	if root == "" {
		root = filepath.Dir(filepath.Dir(p.dataDir))
	}
	data, e := os.ReadFile(filepath.Join(root, "mcp-bridge-port"))
	if e != nil {
		return resultOf(nil, errTunnelSession)
	}
	port, e := strconv.Atoi(strings.TrimSpace(string(data)))
	if e != nil || port < 1 || port > 65535 {
		return resultOf(nil, errTunnelSession)
	}
	forwarded := cloneMap(args)
	forwarded["_dbxXxlForwarded"] = true
	body := jsonBytes(map[string]any{"plugin_id": pluginID, "connection_id": id, "tool": tool, "arguments": forwarded, "timeout_ms": 45000})
	client := &http.Client{Timeout: 50 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	request, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/call-plugin-tool", port), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, e := client.Do(request)
	if e != nil {
		return bridgeFailure("DBX 桌面桥请求失败，请检查工作台会话")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return bridgeFailure(fmt.Sprintf("DBX 桌面桥请求失败（HTTP %d），请检查会话与 MCP 授权", response.StatusCode))
	}
	var result toolResult
	if e = json.NewDecoder(io.LimitReader(response.Body, 256*1024)).Decode(&result); e != nil || result.Content == nil {
		return bridgeFailure("桌面桥响应无效")
	}
	return result
}
func (p *plugin) queryTool(s *session, name string, args map[string]any) (any, error) {
	if e := s.refreshAccess(); e != nil {
		return nil, e
	}
	form := map[string]any{}
	for from, to := range map[string]string{"offset": "start", "limit": "length", "executor_id": "jobGroup", "job_id": "jobId", "log_id": "logId", "from_line": "fromLineNum", "from": "from", "to": "to"} {
		if v, ok := args[from]; ok {
			form[to] = v
		}
	}
	switch name {
	case "connection_info":
		return s.operation("xxljob/info", params{})
	case "get_overview":
		if !s.admin {
			return nil, errors.New("全局报表无法验证执行器归属；普通账号请查询已授权执行器的任务和日志")
		}
		now := time.Now()
		from, to := textValue(args["from"]), textValue(args["to"])
		if from == "" && to == "" {
			from = now.AddDate(0, 0, -6).Format("2006-01-02") + " 00:00:00"
			to = now.Format("2006-01-02") + " 23:59:59"
		}
		report, e := s.operation("xxljob/report", params{Form: map[string]any{"startDate": from, "endDate": to}})
		if e != nil {
			return nil, e
		}
		overview, e := s.overview()
		return map[string]any{"overview": overview, "report": report, "timeBasis": "Admin server time"}, e
	case "list_jobs":
		for from, to := range map[string]string{"description": "jobDesc", "handler": "executorHandler", "author": "author"} {
			if v, ok := args[from]; ok {
				form[to] = v
			}
		}
		form["triggerStatus"] = -1
		if args["schedule_status"] == "enabled" {
			form["triggerStatus"] = 1
		}
		if args["schedule_status"] == "stopped" {
			form["triggerStatus"] = 0
		}
		return s.operation("xxljob/jobs", params{Form: form})
	case "get_job", "get_job_diagnostics":
		form["id"] = args["job_id"]
		if name == "get_job_diagnostics" {
			return s.diagnostics(form)
		}
		return s.jobDetails(form)
	case "list_executors":
		rows, e := s.groups()
		if e != nil {
			return nil, e
		}
		offset, limit := int(number(args["offset"])), int(number(args["limit"]))
		if limit == 0 {
			limit = 25
		}
		if offset > len(rows) {
			offset = len(rows)
		}
		end := offset + limit
		if end > len(rows) {
			end = len(rows)
		}
		return map[string]any{"data": rows[offset:end], "total": len(rows)}, nil
	case "get_executor":
		return s.groupDetails(number(args["executor_id"]))
	case "list_logs":
		form["logStatus"] = -1
		for key, value := range map[string]int{"success": 1, "failure": 2, "running": 3} {
			if args["status"] == key {
				form["logStatus"] = value
			}
		}
		return s.logPage(form)
	case "get_log":
		return s.resolveLog(form)
	case "read_log":
		form["limit"] = args["limit"]
		return s.logSnapshot(form)
	case "preview_schedule":
		value, e := s.operation("xxljob/nextTriggerTime", params{Form: map[string]any{"scheduleType": "CRON", "scheduleConf": args["cron"]}})
		return map[string]any{"times": value, "timeBasis": "Admin server time; timezone is not provided by Admin"}, e
	case "list_calls":
		return p.auditList(s.conn.ID)
	}
	return nil, errors.New("未知查询工具")
}
