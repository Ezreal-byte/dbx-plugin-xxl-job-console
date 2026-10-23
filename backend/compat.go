package main

import (
	"fmt"
	"net/url"
)

func legacyAdminAPI(version string) bool { return version == "2.2" || version == "2.3" }

// 2.2 has only jobCron. Keep one job model for UI, snapshots and MCP; the
// DO_NOTHING value represents its fixed expiry behavior, not a writable option.
func normalize22Job(value any) error {
	row, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("2.2.x 任务响应不是对象")
	}
	cron, ok := row["jobCron"].(string)
	if !ok || cron == "" {
		return fmt.Errorf("2.2.x 任务响应缺少 jobCron，请检查连接版本")
	}
	row["scheduleType"], row["scheduleConf"], row["misfireStrategy"] = "CRON", cron, "DO_NOTHING"
	delete(row, "jobCron")
	return nil
}

func (s *session) adaptLegacyPage(path string, page map[string]any, write bool) (any, error) {
	result, err := normalizeLegacyPage(page, write)
	if err != nil {
		return nil, err
	}
	if s.adminVersion == "2.2" && path == "/jobinfo/pageList" {
		for _, row := range page["data"].([]any) {
			if err := normalize22Job(row); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func validate22Schedule(form url.Values) error {
	if form.Get("scheduleType") != "CRON" {
		return fmt.Errorf("XXL-JOB 2.2.x 仅支持 CRON 调度")
	}
	if form.Get("misfireStrategy") != "DO_NOTHING" {
		return fmt.Errorf("XXL-JOB 2.2.x 的调度过期策略固定，不支持自定义")
	}
	return nil
}

// The UI consumes one pagination shape; wire formats remain version-specific.
func normalizeLegacyPage(page map[string]any, write bool) (any, error) {
	total, ok1 := page["recordsTotal"].(float64)
	filtered, ok2 := page["recordsFiltered"].(float64)
	_, ok3 := page["data"].([]any)
	if ok1 && ok2 && ok3 && validCount(total) && validCount(filtered) {
		return page, nil
	}
	return nil, uncertain(write, "旧版分页响应字段缺失")
}

func normalizePage(page map[string]any, write bool) (any, error) {
	total, ok1 := page["total"].(float64)
	rows, ok2 := page["data"].([]any)
	if !ok1 || !ok2 || !validCount(total) {
		return nil, uncertain(write, "新版分页响应字段缺失")
	}
	return map[string]any{"recordsTotal": total, "recordsFiltered": total, "data": rows}, nil
}

// Keep UI and tool field names stable across Admin versions.
func (s *session) adaptPage(path string, page map[string]any, write bool) (any, error) {
	normalized, err := normalizePage(page, write)
	if err != nil {
		return nil, err
	}
	if s.adminVersion == "3.5" {
		key := map[string]string{"/jobgroup/pageList": "title", "/jobinfo/pageList": "jobDesc"}[path]
		if key != "" {
			for _, item := range page["data"].([]any) {
				if row, ok := item.(map[string]any); ok {
					row[key] = row["name"]
					delete(row, "name")
				}
			}
		}
	}
	return normalized, nil
}

func validCount(value float64) bool {
	return value >= 0 && value <= 9007199254740991 && value == float64(int64(value))
}

func (s *session) adaptRequest(method, path string, form url.Values) (string, url.Values, error) {
	if s.adminVersion == "2.3" {
		return path, form, nil
	}
	adapted := url.Values{}
	for key, values := range form {
		adapted[key] = append([]string(nil), values...)
	}
	if s.adminVersion == "2.2" {
		switch method {
		case "xxljob/saveJob":
			if err := validate22Schedule(form); err != nil {
				return "", nil, err
			}
			adapted.Set("jobCron", form.Get("scheduleConf"))
			adapted.Del("scheduleType")
			adapted.Del("scheduleConf")
			adapted.Del("misfireStrategy")
		case "xxljob/nextTriggerTime":
			adapted.Set("cron", form.Get("scheduleConf"))
			adapted.Del("scheduleType")
			adapted.Del("scheduleConf")
		}
		return path, adapted, nil
	}
	switch method {
	case "xxljob/jobs", "xxljob/logs", "xxljob/users":
		adapted.Set("offset", form.Get("start"))
		adapted.Set("pagesize", form.Get("length"))
		adapted.Del("start")
		adapted.Del("length")
		if method == "xxljob/jobs" && path == "/jobinfo/pageList" {
			for _, key := range []string{"jobDesc", "executorHandler", "author"} {
				if _, ok := adapted[key]; !ok {
					adapted.Set(key, "")
				}
			}
		} else if method == "xxljob/logs" && adapted.Get("filterTime") == "" {
			adapted.Set("filterTime", "")
		}
	case "xxljob/saveJob":
		if form.Get("id") == "" {
			path = "/jobinfo/insert"
		}
	case "xxljob/saveGroup":
		if form.Get("id") == "" {
			path = "/jobgroup/insert"
		}
	case "xxljob/saveUser":
		if form.Get("id") == "" {
			path = "/user/insert"
		}
	case "xxljob/removeUser":
		adapted.Set("ids[]", form.Get("id"))
		adapted.Del("id")
		path = "/user/delete"
	case "xxljob/start", "xxljob/stop", "xxljob/removeJob", "xxljob/removeGroup":
		if form.Get("id") == "" {
			return "", nil, fmt.Errorf("%s 缺少任务或执行器 ID", method)
		}
		adapted.Set("ids[]", form.Get("id"))
		adapted.Del("id")
		switch method {
		case "xxljob/removeJob":
			path = "/jobinfo/delete"
		case "xxljob/removeGroup":
			path = "/jobgroup/delete"
		}
	case "xxljob/logContent":
		adapted.Del("executorAddress")
		adapted.Del("triggerTime")
	}
	if s.adminVersion == "3.5" {
		switch path {
		case "/jobgroup/pageList", "/jobgroup/insert", "/jobgroup/update":
			adapted.Set("name", adapted.Get("title"))
			adapted.Del("title")
		case "/jobinfo/pageList", "/jobinfo/insert", "/jobinfo/update":
			adapted.Set("name", adapted.Get("jobDesc"))
			adapted.Del("jobDesc")
		}
	}
	return path, adapted, nil
}
