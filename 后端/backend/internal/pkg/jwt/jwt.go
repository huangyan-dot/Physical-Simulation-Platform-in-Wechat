// Package jwt 提供轻量 HMAC-SHA256 签名 token（不依赖第三方 JWT 库）。
// 格式：base64url(payload).base64url(hmac_sig)，payload = {uid,role,exp}。
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Claims token 载荷
type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"` // unix 秒
}

// Manager 签发/校验 token
type Manager struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, expireHours int) *Manager {
	if expireHours <= 0 {
		expireHours = 72
	}
	return &Manager{secret: []byte(secret), ttl: time.Duration(expireHours) * time.Hour}
}

// Sign 签发 token
func (m *Manager) Sign(uid int64, role string) (string, error) {
	c := Claims{UserID: uid, Role: role, Exp: time.Now().Add(m.ttl).Unix()}
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(body)
	return enc + "." + m.sum(enc), nil
}

// Parse 校验并解析 token
func (m *Manager) Parse(token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token")
	}
	enc, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(m.sum(enc)), []byte(sig)) {
		return nil, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	var c Claims
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, err
	}
	if c.Exp < time.Now().Unix() {
		return nil, errors.New("token expired")
	}
	return &c, nil
}

func (m *Manager) sum(s string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
