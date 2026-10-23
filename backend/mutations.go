package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type mutationPlan struct {
	Method   string
	Form     map[string]any
	Snapshot any
	Target   string
	Before   map[string]any
	Warnings []string
}

func guardedMutation(method string) bool {
	_, ok := mutations[method]
	return ok && method != "xxljob/saveUser" && method != "xxljob/removeUser"
}
func uiTool(method string, form map[string]any) string {
	switch method {
	case "xxljob/start":
		return "start_job"
	case "xxljob/stop":
		return "stop_job"
	case "xxljob/trigger":
		return "trigger_job"
	case "xxljob/removeJob":
		return "delete_job"
	case "xxljob/saveJob":
		if number(form["id"]) > 0 {
			return "update_job"
		}
		return "create_job"
	case "xxljob/saveGroup":
		if number(form["id"]) > 0 {
			return "update_executor"
		}
		return "create_executor"
	case "xxljob/removeGroup":
		return "delete_executor"
	}
	return ""
}
func uiArguments(method string, form map[string]any) map[string]any {
	args := map[string]any{}
	name := uiTool(method, form)
	switch name {
	case "create_job", "update_job", "create_executor", "update_executor":
		fields := cloneMap(form)
		delete(fields, "id")
		delete(fields, "expectedRevision")
		delete(fields, "glueType")
		args["fields"] = fields
	}
	if strings.Contains(name, "executor") {
		if id := number(form["id"]); id > 0 {
			args["executor_id"] = id
		}
	} else {
		if id := number(form["id"]); id > 0 {
			args["job_id"] = id
		}
		if g := number(form["jobGroup"]); g > 0 {
			args["executor_id"] = g
		}
	}
	if name == "trigger_job" {
		args["parameter_mode"] = textValue(form["parameterMode"])
		if args["parameter_mode"] == "" {
			args["parameter_mode"] = "override"
		}
		if v, ok := form["executorParam"]; ok {
			args["executor_param"] = v
		}
		if a := textValue(form["addressList"]); a != "" {
			args["address"] = a
		}
	}
	if revision := textValue(form["expectedRevision"]); revision != "" {
		args["expected_revision"] = revision
	}
	return args
}
func (p *plugin) uiPreview(s *session, form map[string]any) (any, error) {
	method := textValue(form["operation"])
	input, ok := form["arguments"].(map[string]any)
	if !guardedMutation(method) || !ok {
		return nil, errors.New("预览操作无效")
	}
	return p.mutation(s, uiTool(method, input), uiArguments(method, input), "", false)
}
func (p *plugin) uiExecute(s *session, method string, v params) (any, error) {
	if !v.Confirmed || v.PreviewToken == "" {
		return nil, errors.New("需要先预览并确认操作")
	}
	return p.mutation(s, uiTool(method, v.Form), uiArguments(method, v.Form), v.PreviewToken, false)
}
func defaultJob() map[string]any {
	return map[string]any{"scheduleType": "CRON", "scheduleConf": "0 0/5 * * * ?", "misfireStrategy": "DO_NOTHING", "alarmEmail": "", "executorRouteStrategy": "FIRST", "executorParam": "", "executorBlockStrategy": "SERIAL_EXECUTION", "executorTimeout": 0, "executorFailRetryCount": 0, "glueType": "BEAN", "childJobId": ""}
}
func (s *session) planMutation(name string, args map[string]any) (mutationPlan, error) {
	plan := mutationPlan{Before: map[string]any{}, Warnings: []string{}}
	fail := func(e error) (mutationPlan, error) { return plan, e }
	if e := s.active(); e != nil {
		return fail(e)
	}
	if s.readOnly {
		return fail(errors.New("当前连接为只读，禁止修改或执行任务"))
	}
	if e := s.refreshAccess(); e != nil {
		return fail(e)
	}
	fields, _ := args["fields"].(map[string]any)
	id, gid := number(args["job_id"]), number(args["executor_id"])
	if strings.Contains(name, "executor") {
		if !s.admin {
			return fail(errors.New("仅管理员可以管理执行器"))
		}
		form := map[string]any{"addressType": 0, "addressList": ""}
		var before any = map[string]any{}
		if name != "create_executor" {
			g, e := s.findGroup(gid)
			if e != nil {
				return fail(e)
			}
			form = groupSnapshot(g)
			before = cloneMap(form)
		}
		plan.Before = cloneMap(form)
		if revision := textValue(args["expected_revision"]); revision != "" && revision != digest(plan.Before) {
			return fail(errors.New("执行器配置已被修改，请重新打开编辑器"))
		}
		for k, v := range fields {
			form[k] = v
		}
		if name == "update_executor" && form["appname"] != plan.Before["appname"] {
			return fail(errors.New("编辑执行器不能变更 AppName"))
		}
		if name == "delete_executor" {
			page, e := s.page("xxljob/jobs", map[string]any{"jobGroup": gid, "length": 1})
			if e != nil {
				return fail(e)
			}
			if number(page["recordsFiltered"]) != 0 {
				return fail(errors.New("执行器仍有关联任务，禁止删除"))
			}
			plan.Method = "xxljob/removeGroup"
			form = map[string]any{"id": gid}
			plan.Snapshot = map[string]any{"executor": before, "jobCount": 0}
		} else {
			plan.Method = "xxljob/saveGroup"
			plan.Snapshot = before
		}
		plan.Form = form
		plan.Target = fmt.Sprintf("执行器 %v · #%d", plan.Before["title"], gid)
		if name == "create_executor" {
			plan.Target = "新执行器 · " + textValue(form["title"])
		}
	} else {
		var job map[string]any
		if name != "create_job" {
			var e error
			job, e = s.findJob(id, gid)
			if e != nil {
				return fail(e)
			}
			gid = number(job["jobGroup"])
			if revision := textValue(args["expected_revision"]); revision != "" && revision != digest(jobSnapshot(job)) {
				return fail(errors.New("任务配置已被修改，请重新打开编辑器"))
			}
			plan.Before = jobSnapshot(job)
		}
		form := map[string]any{"id": id}
		switch name {
		case "create_job", "update_job", "copy_job":
			form = defaultJob()
			if job != nil {
				if textValue(job["glueType"]) != "BEAN" {
					return fail(errors.New("仅支持创建、编辑和复制 BEAN 任务；GLUE 源码请在 Admin 管理"))
				}
				for k, v := range jobSnapshot(job) {
					form[k] = v
				}
			}
			if name == "copy_job" {
				form["jobDesc"] = textValue(job["jobDesc"]) + " (副本)"
			}
			for k, v := range fields {
				form[k] = v
			}
			form["glueType"] = "BEAN"
			delete(form, "triggerStatus")
			delete(form, "updateTime")
			if name != "update_job" {
				delete(form, "id")
			}
			if !s.permitGroup(int(number(form["jobGroup"]))) {
				return fail(errors.New("目标执行器未授权"))
			}
			targetGroup, e := s.findGroup(number(form["jobGroup"]))
			if e != nil {
				return fail(e)
			}
			plan.Snapshot = map[string]any{"job": jobSnapshot(job), "executor": groupSnapshot(targetGroup)}
			if len(strings.Split(textValue(form["childJobId"]), ",")) > 32 {
				return fail(errors.New("一次最多配置32个子任务"))
			}
			for _, child := range strings.Split(textValue(form["childJobId"]), ",") {
				if child = strings.TrimSpace(child); child != "" {
					childID, e := strconv.ParseInt(child, 10, 32)
					if e != nil || childID <= 0 {
						return fail(errors.New("子任务 ID 无效"))
					}
					if name == "update_job" && childID == id {
						return fail(errors.New("任务不能将自己设为子任务"))
					}
					if _, e = s.findAuthorizedJob(childID); e != nil {
						return fail(e)
					}
				}
			}
			plan.Method = "xxljob/saveJob"
			if name != "update_job" {
				plan.Warnings = append(plan.Warnings, "新任务默认不启用调度")
			}
		case "start_job":
			plan.Method = "xxljob/start"
		case "stop_job":
			plan.Method = "xxljob/stop"
			plan.Warnings = append(plan.Warnings, "只停止后续调度，不终止已经运行的任务")
		case "delete_job":
			plan.Method = "xxljob/removeJob"
			plan.Warnings = append(plan.Warnings, "删除任务无法撤销")
		case "trigger_job":
			mode := textValue(args["parameter_mode"])
			switch mode {
			case "saved":
				if _, ok := args["executor_param"]; ok {
					return fail(errors.New("使用保存参数时不能同时传入覆盖参数"))
				}
				form["executorParam"] = textValue(job["executorParam"])
			case "override":
				value, ok := args["executor_param"].(string)
				if !ok {
					return fail(errors.New("覆盖模式必须明确提供 executor_param"))
				}
				form["executorParam"] = value
				if value == "" {
					plan.Warnings = append(plan.Warnings, "本次执行将明确传入空字符串")
				}
			default:
				return fail(errors.New("请选择使用保存参数或覆盖参数"))
			}
			plan.Method = "xxljob/trigger"
			form["addressList"] = ""
			g, e := s.findGroup(gid)
			if e != nil {
				return fail(e)
			}
			if address := textValue(args["address"]); address != "" {
				if !slices.Contains(registeredNodes(g), address) {
					return fail(errors.New("指定地址不属于当前执行器已注册节点"))
				}
				form["addressList"] = address
			}
			plan.Snapshot = map[string]any{"job": jobSnapshot(job), "nodes": registeredNodes(g)}
			plan.Warnings = append(plan.Warnings, "成功仅表示 Admin 已接收请求，请在调度日志确认执行结果")
		default:
			return fail(errors.New("未知修改工具"))
		}
		plan.Form = form
		if plan.Snapshot == nil {
			plan.Snapshot = jobSnapshot(job)
		}
		plan.Target = fmt.Sprintf("任务 %v · #%d", job["jobDesc"], id)
		if name == "create_job" {
			plan.Target = "新任务 · " + textValue(form["jobDesc"])
		}
	}
	normalized, e := buildForm(plan.Method, plan.Form, s.adminVersion)
	if e != nil {
		return fail(e)
	}
	if e = validateMutation(plan.Method, normalized); e != nil {
		return fail(e)
	}
	if s.adminVersion == "2.2" && plan.Method == "xxljob/saveJob" {
		plan.Warnings = append(plan.Warnings, "Admin 2.2.x 仅支持 Cron，过期策略由服务器固定处理；DO_NOTHING 仅为兼容显示值")
	}
	return plan, nil
}
func (p *plugin) mutation(s *session, name string, args map[string]any, token string, automated bool) (value any, err error) {
	started := time.Now()
	outcome := "rejected"
	resource := textValue(args["job_id"])
	if resource == "" {
		resource = textValue(args["executor_id"])
	}
	defer func() {
		if err != nil && strings.Contains(err.Error(), "结果未知") {
			outcome = "unknown"
		}
		p.audit(s.conn.ID, name, resource, outcome, started)
	}()
	if automated {
		if e := p.policyAllows(s.conn.ID, name); e != nil {
			return nil, e
		}
	}
	// A connection-specific OS lock serializes writes across UI and MCP sidecars.
	release, e := p.lockMutation(s.conn.ID)
	if e != nil {
		return nil, e
	}
	defer release()
	plan, e := s.planMutation(name, args)
	if e != nil {
		return nil, e
	}
	if token == "" {
		key, e := randomID()
		if e != nil {
			return nil, e
		}
		expires := time.Now().UTC().Add(5 * time.Minute)
		e = p.lockedState(true, func(state *automationState) error {
			if len(state.Previews) >= 500 {
				return errors.New("待确认预览过多，请稍后重试")
			}
			state.Previews[key] = previewRecord{s.conn.ID, s.identity, name, digest(args), digest(plan.Snapshot), expires, automated}
			return nil
		})
		if e != nil {
			return nil, e
		}
		changes := []map[string]any{}
		for k, v := range plan.Form {
			if k == "id" || digest(v) == digest(plan.Before[k]) {
				continue
			}
			before, after := plan.Before[k], v
			if automated && (k == "executorParam" || k == "alarmEmail") {
				before = "[已隐藏]"
				after = "[已隐藏]"
			}
			changes = append(changes, map[string]any{"key": k, "before": before, "after": after})
		}
		slices.SortFunc(changes, func(a, b map[string]any) int { return strings.Compare(textValue(a["key"]), textValue(b["key"])) })
		outcome = "preview"
		return map[string]any{"previewToken": key, "expiresAt": expires, "executed": false, "connectionId": s.conn.ID, "isProduction": s.production, "target": plan.Target, "changes": changes, "warnings": plan.Warnings}, nil
	}
	var record previewRecord
	e = p.lockedState(true, func(state *automationState) error {
		r, ok := state.Previews[token]
		if !ok || r.ConnectionID != s.conn.ID || r.Identity != s.identity || r.Operation != name || r.ArgumentsHash != digest(args) || r.Automated != automated {
			return errors.New("预览令牌无效或与连接、操作、参数不一致")
		}
		record = r
		delete(state.Previews, token)
		return nil
	})
	if e != nil {
		return nil, e
	}
	if time.Now().After(record.ExpiresAt) || record.SnapshotHash != digest(plan.Snapshot) {
		return nil, errors.New("预览过期或目标配置已变化，请重新预览")
	}
	if automated {
		if e = p.policyAllows(s.conn.ID, name); e != nil {
			return nil, e
		}
	}
	value, e = s.operation(plan.Method, params{Form: plan.Form, Confirmed: true})
	if e != nil {
		return nil, e
	}
	outcome = "success"
	return map[string]any{"executed": true, "accepted": true, "result": value, "message": func() string {
		if name == "trigger_job" {
			return "Admin 已接收请求，请在调度日志确认执行结果"
		}
		return "操作成功"
	}()}, nil
}
