package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type policy struct {
	Enabled      bool     `json:"enabled"`
	AllowedTools []string `json:"allowedTools"`
	Revision     int      `json:"revision"`
}
type previewRecord struct {
	ConnectionID  string    `json:"connectionId"`
	Identity      string    `json:"identity"`
	Operation     string    `json:"operation"`
	ArgumentsHash string    `json:"argumentsHash"`
	SnapshotHash  string    `json:"snapshotHash"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Automated     bool      `json:"automated"`
}
type auditRecord struct {
	Time         time.Time `json:"time"`
	ConnectionID string    `json:"connectionId"`
	Tool         string    `json:"tool"`
	ResourceID   string    `json:"resourceId,omitempty"`
	DurationMS   int64     `json:"durationMs"`
	Outcome      string    `json:"outcome"`
}
type automationState struct {
	Version  int                      `json:"version"`
	Policies map[string]policy        `json:"policies"`
	Previews map[string]previewRecord `json:"previews"`
	Audit    []auditRecord            `json:"audit"`
}

func jsonBytes(v any) []byte { b, _ := json.Marshal(v); return b }
func digest(v any) string    { sum := sha256.Sum256(jsonBytes(v)); return hex.EncodeToString(sum[:]) }
func connectionIdentity(c connection) string {
	c.Name = ""
	c.ReadOnly = false
	c.IsProduction = false
	c.Config.ReadOnly = false
	c.Config.IsProduction = false
	c.Config.Environment = ""
	return digest(c)
}
func randomID() (string, error) {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}

// Lock a stable file, not the JSON inode replaced by atomic writes.
func (p *plugin) lockedState(write bool, fn func(*automationState) error) error {
	if p.dataDir == "" {
		return errors.New("缺少插件数据目录，AI/MCP 权限已关闭")
	}
	if e := os.MkdirAll(p.dataDir, 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(filepath.Join(p.dataDir, "automation.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	deadline := time.Now().Add(3 * time.Second)
	for !tryFileLock(f) {
		if time.Now().After(deadline) {
			return errors.New("权限文件正被其他进程使用，请重试")
		}
		time.Sleep(15 * time.Millisecond)
	}
	defer unlockFile(f)
	state := automationState{Version: 1, Policies: map[string]policy{}, Previews: map[string]previewRecord{}, Audit: []auditRecord{}}
	path := filepath.Join(p.dataDir, "automation.json")
	dataFile, e := os.Open(path)
	if e == nil {
		data, err := io.ReadAll(io.LimitReader(dataFile, 4*1024*1024+1))
		dataFile.Close()
		if err != nil || len(data) > 4*1024*1024 || json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Policies == nil || state.Previews == nil {
			return errors.New("权限配置损坏或无法读取，AI/MCP 已关闭；请恢复 automation.json")
		}
	} else if !os.IsNotExist(e) {
		return errors.New("无法读取权限配置，AI/MCP 已关闭")
	}
	for key, r := range state.Previews {
		if time.Now().After(r.ExpiresAt) {
			delete(state.Previews, key)
		}
	}
	if e = fn(&state); e != nil {
		return e
	}
	if !write {
		return nil
	}
	if len(state.Audit) > 200 {
		state.Audit = state.Audit[len(state.Audit)-200:]
	}
	data := jsonBytes(state)
	if len(data) > 4*1024*1024 {
		return errors.New("权限记录已达容量上限")
	}
	tmp, e := os.CreateTemp(p.dataDir, ".automation-*.tmp")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(data)
	}
	if e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(tmp.Name(), path)
}
func (p *plugin) policyAllows(id, name string) error {
	return p.lockedState(false, func(s *automationState) error {
		v := s.Policies[id]
		if id == "" || !v.Enabled || !slices.Contains(v.AllowedTools, name) {
			return errors.New("当前连接未授权此 AI/MCP 工具，请在工作台 AI/MCP 权限中配置")
		}
		return nil
	})
}
func (p *plugin) settingsGet(id string) (any, error) {
	if id == "" {
		return nil, errors.New("缺少连接 ID")
	}
	v := policy{AllowedTools: []string{}}
	e := p.lockedState(false, func(s *automationState) error {
		if saved, ok := s.Policies[id]; ok {
			v = saved
		}
		return nil
	})
	out := map[string]any{"connectionId": id, "policy": v, "tools": toolCatalog()}
	if e != nil {
		out["policy"] = policy{AllowedTools: []string{}}
		out["warning"] = e.Error()
	}
	return out, nil
}
func (p *plugin) automationRPC(method string, raw json.RawMessage) (any, error) {
	var in struct {
		ConnectionID string   `json:"connectionId"`
		Enabled      bool     `json:"enabled"`
		AllowedTools []string `json:"allowedTools"`
		Revision     int      `json:"revision"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return nil, errors.New("无效参数")
	}
	if method == "mcp/tools" {
		allowed := map[string]bool{}
		_ = p.lockedState(false, func(s *automationState) error {
			for id, v := range s.Policies {
				if (in.ConnectionID == "" || id == in.ConnectionID) && v.Enabled {
					for _, name := range v.AllowedTools {
						allowed[name] = true
					}
				}
			}
			return nil
		})
		out := []toolDefinition{}
		for _, t := range toolCatalog() {
			if allowed[t.Name] {
				out = append(out, t)
			}
		}
		return map[string]any{"tools": out}, nil
	}
	if method == "mcp/settings/get" {
		return p.settingsGet(in.ConnectionID)
	}
	if method != "mcp/settings/set" {
		return nil, errors.New("未知权限操作")
	}
	p.mu.RLock()
	sess := p.sessions[in.ConnectionID]
	p.mu.RUnlock()
	if sess == nil {
		return nil, errors.New("请先打开该连接的工作台")
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	// Revocation must remain possible after an account expires or becomes read-only.
	if in.Enabled {
		if e := sess.active(); e != nil {
			return nil, e
		}
		if e := sess.refreshAccess(); e != nil {
			return nil, e
		}
	} else {
		in.AllowedTools = nil
	}
	names := []string{}
	for _, name := range in.AllowedTools {
		t, ok := findTool(name)
		if !ok {
			return nil, errors.New("未知工具")
		}
		if (sess.readOnly && t.Group != "query") || (t.Group == "executors" && !sess.admin) {
			return nil, errors.New("当前连接或账号不能授权此修改工具")
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	e := p.lockedState(true, func(s *automationState) error {
		prev := s.Policies[in.ConnectionID]
		if prev.Revision != in.Revision {
			return errors.New("权限已在其他窗口修改，请重新读取")
		}
		if in.Enabled && !prev.Enabled {
			names = []string{}
			for _, t := range toolCatalog() {
				if t.Group == "query" {
					names = append(names, t.Name)
				}
			}
		}
		s.Policies[in.ConnectionID] = policy{in.Enabled, names, prev.Revision + 1}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return p.settingsGet(in.ConnectionID)
}
func (p *plugin) audit(id, tool, resource, outcome string, started time.Time) {
	_ = p.lockedState(true, func(s *automationState) error {
		s.Audit = append(s.Audit, auditRecord{time.Now().UTC(), id, tool, resource, time.Since(started).Milliseconds(), outcome})
		return nil
	})
}
func (p *plugin) auditList(id string) (any, error) {
	out := []auditRecord{}
	e := p.lockedState(false, func(s *automationState) error {
		for i := len(s.Audit) - 1; i >= 0; i-- {
			if s.Audit[i].ConnectionID == id {
				out = append(out, s.Audit[i])
			}
		}
		return nil
	})
	return out, e
}

func (p *plugin) lockMutation(id string) (func(), error) {
	if p.dataDir == "" {
		return nil, errors.New("缺少插件数据目录，无法验证操作预览")
	}
	if e := os.MkdirAll(p.dataDir, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(p.dataDir, "mutation-"+digest(id)+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	deadline := time.Now().Add(3 * time.Second)
	for !tryFileLock(f) {
		if time.Now().After(deadline) {
			f.Close()
			return nil, errors.New("该连接正在执行其他修改，请稍后重试")
		}
		time.Sleep(15 * time.Millisecond)
	}
	return func() { unlockFile(f); f.Close() }, nil
}
