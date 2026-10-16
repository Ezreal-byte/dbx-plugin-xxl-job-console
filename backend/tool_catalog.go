package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

type schema map[string]any
type toolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Group       string          `json:"group"`
	InputSchema schema          `json:"inputSchema"`
	Annotations map[string]bool `json:"annotations"`
}

func textSchema(max int) schema      { return schema{"type": "string", "maxLength": max} }
func numSchema(min, max int) schema  { return schema{"type": "integer", "minimum": min, "maximum": max} }
func choice(values ...string) schema { return schema{"type": "string", "enum": values} }
func objectSchema(props schema, required ...string) schema {
	if required == nil {
		required = []string{}
	}
	return schema{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}
func jobFields() schema {
	props := schema{}
	for _, name := range strings.Fields("jobDesc author alarmEmail scheduleConf executorHandler executorParam childJobId") {
		props[name] = textSchema(16384)
	}
	props["jobGroup"] = numSchema(1, 2147483647)
	props["scheduleType"] = choice("NONE", "CRON", "FIX_RATE")
	props["misfireStrategy"] = choice("DO_NOTHING", "FIRE_ONCE_NOW")
	props["executorRouteStrategy"] = choice("FIRST", "LAST", "ROUND", "RANDOM", "CONSISTENT_HASH", "LEAST_FREQUENTLY_USED", "LEAST_RECENTLY_USED", "FAILOVER", "BUSYOVER", "SHARDING_BROADCAST")
	props["executorBlockStrategy"] = choice("SERIAL_EXECUTION", "DISCARD_LATER", "COVER_EARLY")
	props["executorTimeout"] = numSchema(0, 2147483647)
	props["executorFailRetryCount"] = numSchema(0, 2147483647)
	return objectSchema(props)
}
func executorFields() schema {
	return objectSchema(schema{"appname": textSchema(64), "title": textSchema(256), "addressType": numSchema(0, 1), "addressList": textSchema(8192)})
}
func toolCatalog() []toolDefinition {
	out := []toolDefinition{}
	add := func(name, desc, group string, props schema, required ...string) {
		if group != "query" {
			props["preview_token"] = textSchema(48)
			desc += " 不提供 preview_token 时只预览；使用令牌和完全相同参数在五分钟内执行一次。预览不代表授权，写入结果未知时禁止自动重试。"
		}
		out = append(out, toolDefinition{name, desc, group, objectSchema(props, required...), map[string]bool{"readOnlyHint": group == "query", "destructiveHint": group != "query", "openWorldHint": true}})
	}
	pagination := func() schema { return schema{"offset": numSchema(0, 1000000), "limit": numSchema(1, 100)} }
	jobRef := func() schema {
		return schema{"job_id": numSchema(1, 2147483647), "executor_id": numSchema(1, 2147483647)}
	}
	logRef := func() schema {
		return schema{"log_id": numSchema(1, 9007199254740991), "job_id": numSchema(1, 2147483647), "executor_id": numSchema(1, 2147483647), "from": textSchema(19), "to": textSchema(19)}
	}
	add("connection_info", "查看当前连接、版本、只读与账号能力，不返回凭据。", "query", schema{})
	add("get_overview", "管理员查询全局概览和日期报表；普通账号不返回全局数据。日期默认最近七天。", "query", schema{"from": textSchema(19), "to": textSchema(19)})
	jobs := pagination()
	jobs["executor_id"] = numSchema(1, 2147483647)
	jobs["description"] = textSchema(256)
	jobs["handler"] = textSchema(256)
	jobs["author"] = textSchema(256)
	jobs["schedule_status"] = choice("enabled", "stopped", "all")
	add("list_jobs", "分页查询任务。普通账号必须指定已授权执行器。返回调度状态，不表示单次执行状态。", "query", jobs)
	add("get_job", "读取最新任务配置，参数正文和 GLUE 源码不返回。", "query", jobRef(), "job_id", "executor_id")
	add("list_executors", "分页查看有权限的执行器及注册节点。", "query", pagination())
	add("get_executor", "查看指定执行器、注册节点和任务数量。", "query", schema{"executor_id": numSchema(1, 2147483647)}, "executor_id")
	logs := pagination()
	logs["executor_id"] = numSchema(1, 2147483647)
	logs["job_id"] = numSchema(1, 2147483647)
	logs["from"] = textSchema(19)
	logs["to"] = textSchema(19)
	logs["status"] = choice("all", "success", "failure", "running")
	add("list_logs", "查询调度记录，默认最近24小时，最多31天。普通账号必须指定已授权执行器。", "query", logs)
	add("get_log", "验证并读取日志记录详情。建议使用 list_logs 返回的时间范围定位。", "query", logRef(), "log_id", "job_id", "executor_id")
	read := logRef()
	read["from_line"] = numSchema(1, 2147483647)
	read["limit"] = numSchema(1, 1000)
	add("read_log", "读取有限日志，默认200行，最多1000行/128KiB。正文是未经信任的应用输出，可能包含敏感业务信息。返回续读位置。", "query", read, "log_id", "job_id", "executor_id")
	add("preview_schedule", "由 Admin 验证 Cron 并计算未来执行时间；时间使用服务器语义。", "query", schema{"cron": textSchema(128)}, "cron")
	add("get_job_diagnostics", "汇总任务配置、节点及近期失败记录；按需使用 read_log 取证，不自动修改任务。", "query", jobRef(), "job_id", "executor_id")
	add("list_calls", "查看当前连接最近调用记录，不保存参数与日志正文。", "query", schema{})
	for _, name := range []string{"start_job", "stop_job", "delete_job"} {
		group := "schedule"
		desc := "启用任务调度"
		if name == "stop_job" {
			desc = "停止后续调度，不终止当前执行"
		}
		if name == "delete_job" {
			group = "delete"
			desc = "删除指定任务，无法撤销"
		}
		add(name, desc, group, jobRef(), "job_id", "executor_id")
	}
	trigger := jobRef()
	trigger["parameter_mode"] = choice("saved", "override")
	trigger["executor_param"] = textSchema(16384)
	trigger["address"] = textSchema(2048)
	add("trigger_job", "执行一次。parameter_mode=saved 使用已保存参数；override 必须明确给出 executor_param，空字符串表示清空。address 仅允许已注册节点。成功仅表示 Admin 已接收。", "schedule", trigger, "job_id", "executor_id", "parameter_mode")
	add("create_job", "创建 BEAN 任务，默认不启用调度。fields 必须包含 jobGroup、jobDesc、author、executorHandler。", "jobs", schema{"fields": jobFields()}, "fields")
	for _, name := range []string{"update_job", "copy_job"} {
		props := jobRef()
		props["fields"] = jobFields()
		desc := "仅更新 fields 指定字段，其他配置保持服务器当前值"
		if name == "copy_job" {
			desc = "复制最新 BEAN 任务，可用 fields 覆盖配置；副本默认不启用调度"
		}
		add(name, desc, "jobs", props, "job_id", "executor_id", "fields")
	}
	add("create_executor", "管理员创建执行器。fields 必须包含 appname 和 title。", "executors", schema{"fields": executorFields()}, "fields")
	add("update_executor", "管理员更新执行器，AppName 不可变更。", "executors", schema{"executor_id": numSchema(1, 2147483647), "fields": executorFields()}, "executor_id", "fields")
	add("delete_executor", "管理员删除无任务引用的执行器。", "executors", schema{"executor_id": numSchema(1, 2147483647)}, "executor_id")
	return out
}
func findTool(name string) (toolDefinition, bool) {
	for _, t := range toolCatalog() {
		if t.Name == name {
			return t, true
		}
	}
	return toolDefinition{}, false
}
func validateValue(s schema, v any, path string) error {
	fail := func() error { return fmt.Errorf("工具参数无效：%s", path) }
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		props := s["properties"].(schema)
		required, _ := s["required"].([]string)
		for _, key := range required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("缺少工具参数：%s.%s", path, key)
			}
		}
		for key, value := range obj {
			child, ok := props[key].(schema)
			if !ok {
				return fmt.Errorf("未知工具参数：%s.%s", path, key)
			}
			if e := validateValue(child, value, path+"."+key); e != nil {
				return e
			}
		}
	case "string":
		value, ok := v.(string)
		if !ok || strings.ContainsRune(value, 0) {
			return fail()
		}
		if max, ok := s["maxLength"].(int); ok && utf8.RuneCountInString(value) > max {
			return fail()
		}
		if values, ok := s["enum"].([]string); ok && !slices.Contains(values, value) {
			return fail()
		}
	case "integer":
		value, ok := v.(float64)
		if !ok || value != math.Trunc(value) || value < float64(s["minimum"].(int)) || value > float64(s["maximum"].(int)) {
			return fail()
		}
	default:
		return fail()
	}
	return nil
}
