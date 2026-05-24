package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SMSRequest 发送短信验证码请求
type SMSRequest struct {
	HotelID int32  `json:"hotelId"`
	Phone   string `json:"phone"`
	IP      string `json:"ip"`
	MAC     string `json:"mac"`
}

// VerifyRequest 验证并连网放行请求
type VerifyRequest struct {
	HotelID   int32  `json:"hotelId"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	ClientURL string `json:"client_url"`
}

// HandlePortalConfig 获取当前酒店的 Portal 自定义显示参数
func HandlePortalConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"仅支持 GET 请求"}`, http.StatusMethodNotAllowed)
		return
	}

	hotelIdStr := r.URL.Query().Get("hotelId")
	if hotelIdStr == "" {
		http.Error(w, `{"error":"缺少 hotelId 参数"}`, http.StatusBadRequest)
		return
	}

	hotelId, err := strconv.ParseInt(hotelIdStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"hotelId 格式错误"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hotel, err := GetHotelByHotelID(ctx, int32(hotelId))
	if err != nil {
		http.Error(w, `{"error":"数据库查询失败"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if hotel == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"未找到对应酒店配置"}`))
		return
	}

	// 只返回 Portal 界面需要的脱敏数据，保障 app_key 安全
	respData := map[string]interface{}{
		"hotelId":      hotel.HotelID,
		"name":         hotel.Name,
		"welcome_text": hotel.WelcomeText,
		"status":       hotel.Status,
		"bypass_auth":  hotel.BypassAuth,
	}

	json.NewEncoder(w).Encode(respData)
}

// HandleHotelRedirect 路由直连：直接渲染并服务 web/portal/index.html 页面，不再执行 302 重定向，保证浏览器地址栏 URL 维持原样
func HandleHotelRedirect(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/hotel/") {
		http.NotFound(w, r)
		return
	}

	hotelIdStr := strings.TrimPrefix(path, "/hotel/")
	// 如果包含斜杠后缀，去除后缀以提取干净的数字 ID
	hotelIdStr = strings.TrimSuffix(hotelIdStr, "/")
	if hotelIdStr == "" {
		http.Error(w, "未指定酒店ID", http.StatusBadRequest)
		return
	}

	hotelId, err := strconv.ParseInt(hotelIdStr, 10, 32)
	if err != nil {
		http.Error(w, "酒店ID格式错误", http.StatusBadRequest)
		return
	}

	// 校验酒店在数据库中是否存在
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hotel, err := GetHotelByHotelID(ctx, int32(hotelId))
	if err != nil {
		http.Error(w, "数据库查询失败", http.StatusInternalServerError)
		return
	}
	if hotel == nil {
		http.Error(w, "未找到对应酒店配置", http.StatusNotFound)
		return
	}

	// 从嵌入的静态资源中直接读取并返回 captive portal 的主页面 HTML
	indexBytes, err := embeddedWebFS.ReadFile("web/portal/index.html")
	if err != nil {
		http.Error(w, "静态资源读取失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexBytes)
}


// HandleSMSReleaseSend 连网验证码发送入口 (需执行严格的安全频次检测与双层计费阻断)
func HandleSMSReleaseSend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req SMSRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"JSON 格式解析错误"}`))
		return
	}

	if req.Phone == "" || req.HotelID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"缺少必要参数"}`))
		return
	}

	// 补全 IP (从请求头读取)
	clientIP := req.IP
	if clientIP == "" {
		clientIP = getClientIP(r)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 0. 优先获取酒店配置以解析限流参数
	hotel, err := GetHotelByHotelID(ctx, req.HotelID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"数据库查询失败"}`))
		return
	}
	if hotel == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"未找到对应酒店配置"}`))
		return
	}

	// 1. 严格的安全流控检查 (基于 Redis，动态解析酒店独立限流参数)
	resolvedCfg := ResolveHotelSecurityConfig(hotel, &GlobalConfig.Security)
	err = AllowSend(ctx, req.Phone, clientIP, resolvedCfg)
	if err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}

	// 2. 产生 6 位纯数字验证码
	code := GenerateRandomCode(6)

	// 3. 进入双层计费判定并投递短信
	err = SendSMSAndBill(ctx, req.HotelID, req.Phone, clientIP, code)
	if err != nil {
		w.WriteHeader(http.StatusPaymentRequired) // 402 Payment Required 经典语义
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}

	// 4. 返回发送成功
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"验证码已发送至您的手机，5分钟内有效"}`))
}

// HandleSMSReleaseVerify 验证并计算网关放行算签链接
func HandleSMSReleaseVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req VerifyRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"JSON 格式解析错误"}`))
		return
	}

	if req.Phone == "" || req.Code == "" || req.HotelID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"缺少必要参数"}`))
		return
	}

	clientIP := req.IP
	if clientIP == "" {
		clientIP = getClientIP(r)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 0. 优先加载酒店配置
	hotel, err := GetHotelByHotelID(ctx, req.HotelID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"数据库查询失败"}`))
		return
	}
	if hotel == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"酒店网关配置不存在"}`))
		return
	}

	// 1. 在 Redis 中进行 OTP 短信匹配验证 (动态解析酒店安全参数)
	resolvedCfg := ResolveHotelSecurityConfig(hotel, &GlobalConfig.Security)
	err = VerifyCode(ctx, req.Phone, req.Code, resolvedCfg)
	if err != nil {
		// 写入失败审计日志
		SaveAuthLog(ctx, req.HotelID, req.Phone, req.MAC, clientIP, "failed")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}

	// 3. 根据路由器网关驱动进行标准算签
	redirectURL, err := calculateGatewayRedirect(hotel, req.Phone, req.MAC, clientIP, req.ClientURL)
	if err != nil {
		SaveAuthLog(ctx, req.HotelID, req.Phone, req.MAC, clientIP, "failed_sign")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"计算网关授权算签失败: ` + err.Error() + `"}`))
		return
	}

	// 4. 写入成功放行审计日志
	SaveAuthLog(ctx, req.HotelID, req.Phone, req.MAC, clientIP, "success")

	// 5. 返回放行链接与用于免密免认证的加密盐 bypass_token
	hasher := md5.New()
	hasher.Write([]byte(fmt.Sprintf("%s_%d_bypass_salt_93jf8", req.Phone, req.HotelID)))
	bypassToken := hex.EncodeToString(hasher.Sum(nil))

	respData := map[string]string{
		"status":       "success",
		"redirect_url": redirectURL,
		"phone":        req.Phone,
		"bypass_token": bypassToken,
	}

	json.NewEncoder(w).Encode(respData)
}

// -------------------------------------------------------------
// 各网关协议算签适配器引擎 (Drivers)
// -------------------------------------------------------------

func calculateGatewayRedirect(hotel *Hotel, phone, mac, ip, clientURL string) (string, error) {
	// 对 IP / MAC 进行标准规整，去除可能导致 Nginx 400 RFC Strict 字符
	mac = strings.ReplaceAll(mac, ":", "-")
	mac = strings.ToUpper(mac)

	switch strings.ToLower(hotel.GatewayType) {
	case "ikuai":
		// === 爱快 IKuai 算签算法 ===
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		
		// 算签公式: md5("user_ip=<ip>&timestamp=<timestamp>&mac=<mac>&upload=0&download=0&key=<app_key>")
		rawSignStr := fmt.Sprintf("user_ip=%s&timestamp=%s&mac=%s&upload=0&download=0&key=%s",
			ip, ts, mac, hotel.AppKey)
		
		hasher := md5.New()
		hasher.Write([]byte(rawSignStr))
		token := hex.EncodeToString(hasher.Sum(nil))

		// 构建标准跳转参数 (URL-encode 各个属性，严控非法字符溢出)
		redirectURL := fmt.Sprintf("https://portal.ikuai8-wifi.com/Action/webauth-up?type=20&user_id=1020004_%s&custom_name=%s&user_ip=%s&timestamp=%s&mac=%s&upload=0&download=0&token=%s&release_type=1",
			phone,
			url.QueryEscape(hotel.CustomName),
			url.QueryEscape(ip),
			url.QueryEscape(ts),
			url.QueryEscape(mac),
			token,
		)
		return redirectURL, nil

	case "panabit":
		// === Panabit 算签跳转算法 ===
		// 典型 Panabit 协议表单跳转或直接 GET
		redirectURL := fmt.Sprintf("http://1.1.1.1:800/login?username=%s&password=%s&mac=%s&ip=%s",
			phone, url.QueryEscape(phone), url.QueryEscape(mac), url.QueryEscape(ip))
		return redirectURL, nil

	case "mikrotik":
		// === MikroTik RouterOS 算签跳转算法 ===
		// MikroTik 重定向至本地 /login 路由，并通过表单提交
		redirectURL := fmt.Sprintf("http://192.168.88.1/login?username=%s&dst=%s",
			phone, url.QueryEscape(clientURL))
		return redirectURL, nil

	default:
		// === 默认放行机制 (标准 200 / 无缝直接直连放行) ===
		return clientURL, nil
	}
}

// -------------------------------------------------------------
// 辅助方法
// -------------------------------------------------------------

func getClientIP(r *http.Request) string {
	// 从负载均衡头 X-Forwarded-For 中解出真实用户 IP
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}
	// 备用 fallback
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// BypassRequest 快速二次免短信上线请求结构
type BypassRequest struct {
	HotelID     int32  `json:"hotelId"`
	Phone       string `json:"phone"`
	BypassToken string `json:"bypass_token"`
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	ClientURL   string `json:"client_url"`
}

// HandleSMSReleaseBypass 处理二次免短信直接认证放行
func HandleSMSReleaseBypass(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req BypassRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"JSON 格式解析错误"}`))
		return
	}

	if req.Phone == "" || req.BypassToken == "" || req.HotelID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"缺少必要校验参数"}`))
		return
	}

	clientIP := req.IP
	if clientIP == "" {
		clientIP = getClientIP(r)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 加载酒店网关配置
	hotel, err := GetHotelByHotelID(ctx, req.HotelID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"数据库查询失败"}`))
		return
	}
	if hotel == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"酒店网关配置不存在"}`))
		return
	}

	// 2. 检查该酒店是否启用了免短信认证
	if hotel.BypassAuth != 1 {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"当前酒店未启用二次免短信快捷认脸上网功能，请进行短信验证。"}`))
		return
	}

	// 3. 计算并检验客户端 BypassToken 与 hotelId 盐是否匹配
	hasher := md5.New()
	hasher.Write([]byte(fmt.Sprintf("%s_%d_bypass_salt_93jf8", req.Phone, req.HotelID)))
	expectedToken := hex.EncodeToString(hasher.Sum(nil))

	if expectedToken != req.BypassToken {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"免验证口令校验失败，该手机号免密授权可能已过期或盐参数已变更，请使用短信验证码登录"}`))
		return
	}

	// 4. 计算网关放行跳转地址
	redirectURL, err := calculateGatewayRedirect(hotel, req.Phone, req.MAC, clientIP, req.ClientURL)
	if err != nil {
		SaveAuthLog(ctx, req.HotelID, req.Phone, req.MAC, clientIP, "failed_bypass_sign")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"计算免短信网关授权算签失败: ` + err.Error() + `"}`))
		return
	}

	// 5. 写入成功免登日志
	SaveAuthLog(ctx, req.HotelID, req.Phone, req.MAC, clientIP, "success_bypass")

	// 6. 返回重定向地址
	respData := map[string]string{
		"status":       "success",
		"redirect_url": redirectURL,
	}
	json.NewEncoder(w).Encode(respData)
}
