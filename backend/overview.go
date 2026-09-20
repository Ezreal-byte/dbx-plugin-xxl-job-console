package main

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Both supported Admin versions render these three dashboardInfo values in order.
func parseOverview(raw []byte) (map[string]int, error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	values := []int{}
	visit(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "span" {
			return
		}
		for _, class := range strings.Fields(attrs(n)["class"]) {
			if class == "info-box-number" {
				value, parseErr := strconv.Atoi(strings.ReplaceAll(text(n), ",", ""))
				if parseErr == nil && value >= 0 {
					values = append(values, value)
				}
				break
			}
		}
	})
	if len(values) < 3 {
		return nil, errors.New("未找到调度中心概览指标")
	}
	return map[string]int{"jobs": values[0], "triggers": values[1], "executors": values[2]}, nil
}

func (s *session) overview() (map[string]int, error) {
	path := "/"
	if s.adminVersion == "3.4" {
		path = "/dashboard"
	}
	raw, err := s.raw("GET", path, nil, false)
	if err != nil {
		return nil, err
	}
	return parseOverview(raw)
}
