package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const scanLimit = 2000

var errIncomplete = errors.New("扫描达到上限，结果不完整；请缩小执行器或时间范围，不能据此判断资源不存在")

func number(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}
func textValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func cloneMap(v map[string]any) map[string]any {
	out := map[string]any{}
	for k, x := range v {
		out[k] = x
	}
	return out
}
func (s *session) page(method string, form map[string]any) (map[string]any, error) {
	v, e := s.operation(method, params{Form: form})
	if e != nil {
		return nil, e
	}
	page, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("分页响应无效")
	}
	return page, nil
}
func (s *session) findJob(id, groupID int64) (map[string]any, error) {
	if id < 1 {
		return nil, errors.New("任务 ID 无效")
	}
	if e := s.refreshAccess(); e != nil {
		return nil, e
	}
	if groupID < 1 && !s.admin {
		return nil, errors.New("请指定已授权执行器")
	}
	if groupID < 1 {
		groupID = -1
	}
	if !s.permitGroup(int(groupID)) {
		return nil, errors.New("没有该执行器权限")
	}
	for offset := 0; offset < scanLimit; offset += 100 {
		page, e := s.page("xxljob/jobs", map[string]any{"jobGroup": groupID, "start": offset, "length": 100})
		if e != nil {
			return nil, e
		}
		rows := page["data"].([]any)
		for _, v := range rows {
			row := v.(map[string]any)
			if number(row["id"]) == id {
				if groupID > 0 && number(row["jobGroup"]) != groupID {
					return nil, errors.New("任务所属执行器不一致")
				}
				return cloneMap(row), nil
			}
		}
		if int64(offset+len(rows)) >= number(page["recordsFiltered"]) {
			return nil, errors.New("任务不存在或不在授权范围，请刷新")
		}
		if len(rows) == 0 {
			return nil, errIncomplete
		}
	}
	return nil, errIncomplete
}
func (s *session) findGroup(id int64) (group, error) {
	if id < 1 {
		return group{}, errors.New("执行器 ID 无效")
	}
	rows, e := s.groups()
	if e != nil {
		return group{}, e
	}
	for _, g := range rows {
		if int64(g.ID) == id {
			return g, nil
		}
	}
	return group{}, errors.New("执行器不存在或未授权")
}

// Resolve child jobs across only authorized executors with one shared row budget.
func (s *session) findAuthorizedJob(id int64) (map[string]any, error) {
	if s.admin {
		return s.findJob(id, 0)
	}
	groups := append([]group{}, s.allowed...)
	scanned := 0
	for _, g := range groups {
		for offset := 0; ; offset += 100 {
			if scanned >= scanLimit {
				return nil, errIncomplete
			}
			page, e := s.page("xxljob/jobs", map[string]any{"jobGroup": g.ID, "start": offset, "length": 100})
			if e != nil {
				return nil, e
			}
			rows := page["data"].([]any)
			scanned += len(rows)
			for _, v := range rows {
				row := v.(map[string]any)
				if number(row["id"]) == id {
					return cloneMap(row), nil
				}
			}
			if int64(offset+len(rows)) >= number(page["recordsFiltered"]) {
				break
			}
			if len(rows) == 0 {
				return nil, errIncomplete
			}
		}
	}
	return nil, errors.New("子任务不存在或不在授权范围")
}
func (s *session) groupDetails(id int64) (any, error) {
	g, e := s.findGroup(id)
	if e != nil {
		return nil, e
	}
	page, e := s.page("xxljob/jobs", map[string]any{"jobGroup": id, "length": 1})
	if e != nil {
		return nil, e
	}
	return map[string]any{"executor": g, "revision": digest(groupSnapshot(g)), "jobCount": page["recordsFiltered"], "nodes": registeredNodes(g)}, nil
}
func groupSnapshot(g group) map[string]any {
	return map[string]any{"id": g.ID, "appname": g.Appname, "title": g.Title, "addressType": g.AddressType, "addressList": g.AddressList}
}
func registeredNodes(g group) []string {
	out := []string{}
	for _, a := range strings.Split(g.AddressList, ",") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}
func jobSnapshot(job map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range strings.Fields("id jobGroup jobDesc author alarmEmail scheduleType scheduleConf misfireStrategy executorRouteStrategy executorHandler executorParam executorBlockStrategy executorTimeout executorFailRetryCount glueType childJobId triggerStatus updateTime") {
		if v, ok := job[k]; ok {
			out[k] = v
		}
	}
	return out
}
func (s *session) jobDetails(form map[string]any) (any, error) {
	job, e := s.findJob(number(form["id"]), number(form["jobGroup"]))
	if e != nil {
		return nil, e
	}
	g, e := s.findGroup(number(job["jobGroup"]))
	if e != nil {
		return nil, e
	}
	out := map[string]any{"job": job, "revision": digest(jobSnapshot(job)), "executor": g, "nodes": registeredNodes(g), "recentLogs": []any{}, "partial": false}
	logs, e := s.logPage(map[string]any{"jobGroup": job["jobGroup"], "jobId": job["id"], "length": 10})
	if e != nil {
		out["partial"] = true
		out["warning"] = e.Error()
	} else {
		out["recentLogs"] = logs["data"]
		out["recentLogCount"] = logs["recordsFiltered"]
	}
	return out, nil
}
func (s *session) diagnostics(form map[string]any) (any, error) {
	value, e := s.jobDetails(form)
	if e != nil {
		return nil, e
	}
	out := value.(map[string]any)
	job := out["job"].(map[string]any)
	failures := []any{}
	for _, v := range out["recentLogs"].([]any) {
		row := v.(map[string]any)
		if status := logStatus(row); status == "failure" || status == "trigger_failed" {
			failures = append(failures, row)
		}
	}
	out["diagnostics"] = map[string]any{"scheduleEnabled": number(job["triggerStatus"]) == 1, "registeredNodeCount": len(out["nodes"].([]string)), "recentFailures": failures, "windowHours": 24, "sampleLimit": 10, "sampled": number(out["recentLogCount"]) > 10, "logBodiesIncluded": false}
	return out, nil
}
func logRange(form map[string]any) (string, error) {
	now := time.Now()
	from := textValue(form["from"])
	to := textValue(form["to"])
	if from == "" && to == "" {
		return now.Add(-24*time.Hour).Format("2006-01-02 15:04:05") + " - " + now.Format("2006-01-02 15:04:05"), nil
	}
	a, e1 := time.ParseInLocation("2006-01-02 15:04:05", from, time.Local)
	b, e2 := time.ParseInLocation("2006-01-02 15:04:05", to, time.Local)
	if e1 != nil || e2 != nil || b.Before(a) || b.Sub(a) > 31*24*time.Hour {
		return "", errors.New("日志时间范围必须完整，且不超过31天")
	}
	return from + " - " + to, nil
}
func (s *session) logPage(form map[string]any) (map[string]any, error) {
	f := cloneMap(form)
	r, e := logRange(f)
	if e != nil {
		return nil, e
	}
	f["filterTime"] = r
	return s.page("xxljob/logs", f)
}
func (s *session) resolveLog(form map[string]any) (map[string]any, error) {
	id, jobID, groupID := number(form["logId"]), number(form["jobId"]), number(form["jobGroup"])
	if id < 1 || jobID < 1 || groupID < 1 {
		return nil, errors.New("日志、任务及执行器 ID 必须是正整数")
	}
	f := cloneMap(form)
	f["length"] = 100
	for offset := 0; offset < scanLimit; offset += 100 {
		f["start"] = offset
		page, e := s.logPage(f)
		if e != nil {
			return nil, e
		}
		rows := page["data"].([]any)
		for _, v := range rows {
			row := v.(map[string]any)
			if number(row["id"]) == id {
				if number(row["jobId"]) != jobID || number(row["jobGroup"]) != groupID {
					return nil, errors.New("日志身份不匹配")
				}
				return cloneMap(row), nil
			}
		}
		if int64(offset+len(rows)) >= number(page["recordsFiltered"]) {
			return nil, errors.New("指定时间范围内未找到日志，请调整范围后重试")
		}
		if len(rows) == 0 {
			return nil, errIncomplete
		}
	}
	return nil, errIncomplete
}
func (s *session) logSnapshot(form map[string]any) (any, error) {
	row, e := s.resolveLog(form)
	if e != nil {
		return nil, e
	}
	from := number(form["fromLineNum"])
	if from == 0 {
		from = 1
	}
	limit := number(form["limit"])
	if limit == 0 {
		limit = 200
	}
	if from < 1 || limit < 1 || limit > 1000 {
		return nil, errors.New("日志行数超出范围")
	}
	ms, e := logMillis(row["triggerTime"])
	if e != nil {
		return nil, e
	}
	value, e := s.operation("xxljob/logContent", params{Form: map[string]any{"logId": row["id"], "fromLineNum": from, "executorAddress": row["executorAddress"], "triggerTime": ms}})
	if e != nil {
		return nil, e
	}
	chunk, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("日志响应无效")
	}
	body, ok := chunk["logContent"].(string)
	if !ok || number(chunk["fromLineNum"]) != from {
		return nil, errors.New("日志游标不一致")
	}
	lines := strings.SplitAfter(body, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out strings.Builder
	count := 0
	truncated := false
	partialLine := false
	for _, line := range lines {
		if int64(count) >= limit {
			truncated = true
			break
		}
		remaining := 128*1024 - out.Len()
		if len(line) > remaining {
			truncated = true
			if count == 0 {
				part := line[:remaining]
				for !utf8.ValidString(part) && len(part) > 0 {
					part = part[:len(part)-1]
				}
				out.WriteString(part)
				count++
				partialLine = true
			}
			break
		}
		out.WriteString(line)
		count++
	}
	end, _ := chunk["end"].(bool)
	next := from + int64(count)
	return map[string]any{"logId": row["id"], "jobId": row["jobId"], "fromLineNum": from, "toLineNum": next - 1, "nextLine": next, "logContent": out.String(), "end": end && !truncated, "truncated": truncated, "partialLine": partialLine, "executionStatus": logStatus(row)}, nil
}
func logStatus(row map[string]any) string {
	if number(row["triggerCode"]) == 0 {
		return "trigger_pending"
	}
	if number(row["triggerCode"]) != 200 {
		return "trigger_failed"
	}
	switch number(row["handleCode"]) {
	case 200:
		return "success"
	case 0:
		return "running"
	default:
		return "failure"
	}
}
func (s *session) resourceQuery(method string, form map[string]any) (any, bool, error) {
	var value any
	var e error
	switch method {
	case "xxljob/jobDetails":
		value, e = s.jobDetails(form)
	case "xxljob/diagnostics":
		value, e = s.diagnostics(form)
	case "xxljob/executorDetails":
		value, e = s.groupDetails(number(form["id"]))
	case "xxljob/resolveLog":
		value, e = s.resolveLog(form)
	case "xxljob/logSnapshot":
		value, e = s.logSnapshot(form)
	default:
		return nil, false, nil
	}
	return value, true, e
}
