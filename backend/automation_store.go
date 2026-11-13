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
	"time"
)

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
		return errors.New("缺少插件数据目录，无法读写操作预览")
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
			return errors.New("预览与审计状态正被其他进程使用，请重试")
		}
		time.Sleep(15 * time.Millisecond)
	}
	defer unlockFile(f)
	state := automationState{Version: 1, Previews: map[string]previewRecord{}, Audit: []auditRecord{}}
	path := filepath.Join(p.dataDir, "automation.json")
	dataFile, e := os.Open(path)
	if e == nil {
		data, err := io.ReadAll(io.LimitReader(dataFile, 4*1024*1024+1))
		dataFile.Close()
		if err != nil || len(data) > 4*1024*1024 || json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Previews == nil {
			return errors.New("预览与审计记录损坏或无法读取；请恢复 automation.json")
		}
	} else if !os.IsNotExist(e) {
		return errors.New("无法读取预览与审计记录")
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
		return errors.New("预览与审计记录已达容量上限")
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
