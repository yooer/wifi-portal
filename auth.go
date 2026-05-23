package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SessionPayload 存储在 Cookie 中的商户会话数据
type SessionPayload struct {
	User     int64 `json:"user"`
	Level    int32 `json:"level"` // 1-100. level > 50 为超级管理员，<= 50 为酒店商户
	ExpireAt int64 `json:"exp"`
}

type AuthManager struct {
	secretKey []byte
}

var GlobalAuth *AuthManager

func InitAuth() {
	// 动态生成一个 32 字节的会话密钥，确保每次服务器重启时老会话自动失效，保障高安全性
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	GlobalAuth = &AuthManager{secretKey: key}
}

// CreateSession 创建加密会话 Cookie
func (a *AuthManager) CreateSession(w http.ResponseWriter, sessionUser *SessionPayload, maxAge int) error {
	sessionUser.ExpireAt = time.Now().Add(time.Duration(maxAge) * time.Second).Unix()
	
	payloadBytes, err := json.Marshal(sessionUser)
	if err != nil {
		return err
	}

	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// 计算 HMAC-SHA256 签名
	mac := hmac.New(sha256.New, a.secretKey)
	mac.Write([]byte(payloadBase64))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	token := payloadBase64 + "." + signature

	cookie := &http.Cookie{
		Name:     "portal_saas_session",
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return nil
}

// DestroySession 销毁会话 Cookie
func (a *AuthManager) DestroySession(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "portal_saas_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// VerifySession 校验并提取商户会话数据
func (a *AuthManager) VerifySession(r *http.Request) (*SessionPayload, error) {
	cookie, err := r.Cookie("portal_saas_session")
	if err != nil {
		return nil, fmt.Errorf("未登录，请先登录后台")
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("非法登录会话凭证")
	}

	payloadBase64, signature := parts[0], parts[1]

	mac := hmac.New(sha256.New, a.secretKey)
	mac.Write([]byte(payloadBase64))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("登录会话已过期或已被篡改")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, fmt.Errorf("会话数据流损坏")
	}

	var payload SessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("解析会话失败")
	}

	if time.Now().Unix() > payload.ExpireAt {
		return nil, fmt.Errorf("您的登录状态已过期，请重新登录")
	}

	return &payload, nil
}

// RequireAuth 强制后台商户/管理员登录校验中间件
func (a *AuthManager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := a.VerifySession(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"` + err.Error() + `"}`))
			return
		}

		// 将商户账号信息和权限级别置于 Request Header 中传递给后续 Handler 消费
		r.Header.Set("X-User-Account", strconv.FormatInt(payload.User, 10))
		r.Header.Set("X-User-Level", strconv.Itoa(int(payload.Level)))

		next.ServeHTTP(w, r)
	}
}

// RequireAdmin 强制超级管理员级别校验中间件
func (a *AuthManager) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := a.VerifySession(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"` + err.Error() + `"}`))
			return
		}

		if payload.Level < 50 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"无权访问，本接口仅限超级管理员操作"}`))
			return
		}

		r.Header.Set("X-User-Account", strconv.FormatInt(payload.User, 10))
		r.Header.Set("X-User-Level", strconv.Itoa(int(payload.Level)))

		next.ServeHTTP(w, r)
	}
}
