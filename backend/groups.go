package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type group struct {
	ID          int    `json:"id"`
	Appname     string `json:"appname"`
	Title       string `json:"title"`
	AddressType int    `json:"addressType"`
	AddressList string `json:"addressList"`
}

func attrs(n *html.Node) map[string]string {
	a := map[string]string{}
	for _, v := range n.Attr {
		a[strings.ToLower(v.Key)] = v.Val
	}
	return a
}
func visit(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		visit(c, fn)
	}
}
func text(n *html.Node) string {
	var b strings.Builder
	visit(n, func(v *html.Node) {
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
	})
	return strings.TrimSpace(b.String())
}
func loginHTML(raw []byte) bool {
	if !bytes.Contains(raw, []byte("<")) {
		return false
	}
	doc, e := html.Parse(bytes.NewReader(raw))
	if e != nil {
		return false
	}
	found := false
	visit(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" && attrs(n)["id"] == "loginForm" {
			found = true
		}
	})
	return found
}
func (s *session) refreshAccess() error {
	raw, e := s.raw("GET", "/", nil, false)
	if e != nil {
		return e
	}
	doc, e := html.Parse(bytes.NewReader(raw))
	if e != nil {
		return errors.New("无法解析当前用户权限页面")
	}
	base, _ := url.Parse(s.base + "/")
	admin, jobNav := false, false
	visit(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		href, e := url.Parse(attrs(n)["href"])
		if e != nil {
			return
		}
		u := base.ResolveReference(href)
		if u.Scheme != base.Scheme || u.Host != base.Host {
			return
		}
		if u.Path == strings.TrimRight(base.Path, "/")+"/jobgroup" {
			admin = true
		}
		if u.Path == strings.TrimRight(base.Path, "/")+"/jobinfo" {
			jobNav = true
		}
	})
	if !admin && !jobNav {
		return errors.New("未找到原版任务导航，服务页面或应用路径不兼容")
	}
	s.admin = admin
	if admin {
		return nil
	}
	raw, e = s.raw("GET", "/jobinfo", nil, false)
	if e != nil {
		return e
	}
	s.allowed, e = parseGroups(raw)
	return e
}
func parseGroups(raw []byte) ([]group, error) {
	doc, e := html.Parse(bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	result := []group{}
	found := false
	var invalid error
	visit(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "select" || attrs(n)["id"] != "jobGroup" {
			return
		}
		found = true
		visit(n, func(o *html.Node) {
			if o.Type != html.ElementNode || o.Data != "option" {
				return
			}
			id, e := strconv.Atoi(attrs(o)["value"])
			if e != nil || id < 1 || text(o) == "" {
				invalid = errors.New("授权执行器选项不合法")
				return
			}
			for _, g := range result {
				if g.ID == id {
					invalid = errors.New("授权执行器 ID 重复")
					return
				}
			}
			result = append(result, group{ID: id, Title: text(o)})
		})
	})
	if !found {
		return nil, errors.New("未找到原版授权执行器列表，请检查账号权限和服务版本")
	}
	return result, invalid
}
func (s *session) groups() ([]group, error) {
	if e := s.refreshAccess(); e != nil {
		return nil, e
	}
	if !s.admin {
		return append([]group{}, s.allowed...), nil
	}
	result := []group{}
	for start := 0; start < 10000; start += 100 {
		form := url.Values{"start": {strconv.Itoa(start)}, "length": {"100"}}
		path, form, e := s.adaptRequest("xxljob/jobs", "/jobgroup/pageList", form)
		if e != nil {
			return nil, e
		}
		value, e := s.request(path, form, false)
		if e != nil {
			return nil, e
		}
		page := value.(map[string]any)
		rows := page["data"].([]any)
		for _, v := range rows {
			row, ok := v.(map[string]any)
			if !ok {
				return nil, errors.New("执行器响应不合法")
			}
			id, ok := row["id"].(float64)
			if !ok || id < 1 || id != float64(int(id)) {
				return nil, errors.New("执行器 ID 不合法")
			}
			app, _ := row["appname"].(string)
			title, _ := row["title"].(string)
			address, _ := row["addressList"].(string)
			mode, ok := row["addressType"].(float64)
			if app == "" || title == "" || !ok || (mode != 0 && mode != 1) {
				return nil, errors.New("执行器字段缺失")
			}
			result = append(result, group{ID: int(id), Appname: app, Title: title, AddressType: int(mode), AddressList: address})
		}
		if float64(len(result)) >= page["recordsFiltered"].(float64) {
			s.allowed = result
			return result, nil
		}
		if len(rows) == 0 {
			return nil, errors.New("执行器分页数据缺失")
		}
	}
	return nil, errors.New("执行器超过 10000 条，请使用官方页面管理")
}
func (s *session) permitGroup(id int) bool {
	if s.admin {
		return true
	}
	for _, g := range s.allowed {
		if g.ID == id {
			return true
		}
	}
	return false
}
func (s *session) jobsByGroup(id int) ([]any, error) {
	if id < 1 || !s.permitGroup(id) {
		return nil, errors.New("没有该执行器的权限，请选择已授权执行器")
	}
	if s.adminVersion == "3.4" {
		result := []any{}
		for offset := 0; offset < 10000; offset += 100 {
			form := url.Values{"offset": {strconv.Itoa(offset)}, "pagesize": {"100"}, "jobGroup": {strconv.Itoa(id)}, "triggerStatus": {"-1"}, "jobDesc": {""}, "executorHandler": {""}, "author": {""}}
			value, e := s.request("/jobinfo/pageList", form, false)
			if e != nil {
				return nil, e
			}
			page := value.(map[string]any)
			rows := page["data"].([]any)
			for _, item := range rows {
				row, ok := item.(map[string]any)
				if !ok || row["jobGroup"] != float64(id) {
					return nil, errors.New("任务执行器归属不合法")
				}
				result = append(result, row)
			}
			if float64(len(result)) >= page["recordsFiltered"].(float64) {
				return result, nil
			}
			if len(rows) == 0 {
				return nil, errors.New("任务分页数据缺失")
			}
		}
		return nil, errors.New("任务超过 10000 条，请使用官方页面管理")
	}
	value, e := s.request("/joblog/getJobsByGroup", url.Values{"jobGroup": {strconv.Itoa(id)}}, false)
	if e != nil {
		return nil, e
	}
	rows, ok := value.([]any)
	if !ok {
		return nil, errors.New("任务选项响应不合法")
	}
	for _, v := range rows {
		row, ok := v.(map[string]any)
		if !ok || row["jobGroup"] != float64(id) {
			return nil, errors.New("任务执行器归属不合法")
		}
	}
	return rows, nil
}
func (s *session) permitJob(id string) error {
	if s.admin {
		return nil
	}
	for _, g := range s.allowed {
		rows, e := s.jobsByGroup(g.ID)
		if e != nil {
			return e
		}
		for _, v := range rows {
			row := v.(map[string]any)
			if n, ok := row["id"].(float64); ok && fmt.Sprint(int64(n)) == id {
				return nil
			}
		}
	}
	return errors.New("任务不属于当前账号授权执行器，禁止操作")
}
