package main

import (
	"errors"
	"net/url"
	"strconv"
)

// Never allow an admin to alter the account used by this connection.
func (s *session) ensureNotCurrentUser(id string) error {
	form := url.Values{"start": {"0"}, "length": {"100"}, "username": {s.username}, "role": {"-1"}}
	path, adapted, err := s.adaptRequest("xxljob/users", "/user/pageList", form)
	if err != nil {
		return err
	}
	result, err := s.request(path, adapted, false)
	if err != nil {
		return err
	}
	page, ok := result.(map[string]any)
	if !ok {
		return errors.New("无法核验当前用户")
	}
	rows, ok := page["data"].([]any)
	if !ok {
		return errors.New("无法核验当前用户")
	}
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok || row["username"] != s.username {
			continue
		}
		if n, ok := row["id"].(float64); ok && strconv.FormatInt(int64(n), 10) == id {
			return errors.New("不能编辑或删除当前登录账号")
		}
	}
	return nil
}
