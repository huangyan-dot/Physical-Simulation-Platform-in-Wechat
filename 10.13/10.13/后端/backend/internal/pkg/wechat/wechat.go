// Package wechat 封装微信小程序服务端接口（jscode2session）。
package wechat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Session jscode2session 返回
type Session struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type Client struct {
	appid  string
	secret string
	http   *http.Client
}

func New(appid, secret string) *Client {
	return &Client{
		appid:  appid,
		secret: secret,
		http:   &http.Client{Timeout: 8 * time.Second},
	}
}

// Code2Session 用 wx.login 的 code 换 openid。
// appid/secret 未配置时返回错误（开发期用 dev_ 后门，不走此方法）。
func (c *Client) Code2Session(code string) (*Session, error) {
	if c.appid == "" || c.secret == "" {
		return nil, fmt.Errorf("wechat appid/secret 未配置")
	}
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		c.appid, c.secret, code,
	)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var s Session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	if s.ErrCode != 0 {
		return nil, fmt.Errorf("wechat: %s", s.ErrMsg)
	}
	return &s, nil
}
