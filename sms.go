package main

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// SMSSender 定义统一的短信发送接口
type SMSSender interface {
	SendSMS(phone, code, signName string) error
}

// BalanceQuerier 定义支持查询账户余额的短信通道接口
type BalanceQuerier interface {
	QueryBalance() (string, error)
}

// ==========================================
// 1. Mock 短信通道
// ==========================================
type MockSender struct {
	signName string
}

func (m *MockSender) SendSMS(phone, code, signName string) error {
	activeSign := signName
	if activeSign == "" {
		activeSign = m.signName
	}
	fmt.Printf("\n============================================\n")
	fmt.Printf("🚀 [%s] [Mock SMS Service] 发送验证码成功！\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("🏢 短信签名: 【%s】\n", activeSign)
	fmt.Printf("📱 目标手机号: %s\n", phone)
	fmt.Printf("💬 验证码内容: %s (5分钟内有效)\n", code)
	fmt.Printf("============================================\n\n")
	return nil
}

func (m *MockSender) QueryBalance() (string, error) {
	return "99999 (模拟测试通道)", nil
}

// ==========================================
// 2. 阿里云短信通道
// ==========================================
type AliyunSender struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

func (a *AliyunSender) SendSMS(phone, code, signName string) error {
	if a.AccessKeyID == "" || a.AccessKeySecret == "" {
		return fmt.Errorf("阿里云短信参数未配置")
	}

	activeSign := signName
	if activeSign == "" {
		activeSign = a.SignName
	}

	params := url.Values{}
	params.Set("AccessKeyId", a.AccessKeyID)
	params.Set("Action", "SendSms")
	params.Set("Format", "JSON")
	params.Set("PhoneNumbers", phone)
	params.Set("RegionId", "cn-hangzhou")
	params.Set("SignName", activeSign)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureNonce", fmt.Sprintf("%d", rand.Int63()))
	params.Set("SignatureVersion", "1.0")
	params.Set("TemplateCode", a.TemplateCode)
	params.Set("TemplateParam", fmt.Sprintf("{\"code\":\"%s\"}", code))
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("Version", "2017-05-25")

	signature := a.calculateSignature(params, a.AccessKeySecret)
	params.Set("Signature", signature)

	requestURL := "https://dysmsapi.aliyuncs.com/?" + params.Encode()
	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("请求阿里云短信服务失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取阿里云响应失败: %v", err)
	}

	var result struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析阿里云响应失败: %v", err)
	}

	if result.Code != "OK" {
		return fmt.Errorf("阿里云短信发送失败: [%s] %s", result.Code, result.Message)
	}

	return nil
}

func (a *AliyunSender) calculateSignature(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sortKeys(keys)

	var canonicalizedQueryString string
	for _, k := range keys {
		canonicalizedQueryString += "&" + specialUrlEncode(k) + "=" + specialUrlEncode(params.Get(k))
	}
	canonicalizedQueryString = canonicalizedQueryString[1:]

	stringToSign := "GET&%2F&" + specialUrlEncode(canonicalizedQueryString)

	key := []byte(secret + "&")
	h := hmac.New(sha1.New, key)
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ==========================================
// 3. 腾讯云短信通道
// ==========================================
type TencentSender struct {
	SecretID   string
	SecretKey  string
	SDKAppID   string
	SignName   string
	TemplateID string
}

func (t *TencentSender) SendSMS(phone, code, signName string) error {
	activeSign := signName
	if activeSign == "" {
		activeSign = t.SignName
	}
	log.Printf("[%s] [Tencent SMS Mock] 准备发送验证码 %s 至 %s, 签名: 【%s】", time.Now().Format("2006-01-02 15:04:05"), code, phone, activeSign)
	// 腾讯云此处提供 Mock 实现，确保可以顺利编译
	fmt.Printf("\n============================================\n")
	fmt.Printf("🚀 [%s] [Tencent SMS Service Mock] 发送成功！\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("🏢 短信签名: 【%s】\n", activeSign)
	fmt.Printf("📱 目标手机号: %s\n", phone)
	fmt.Printf("💬 验证码内容: %s (5分钟内有效)\n", code)
	fmt.Printf("============================================\n\n")
	return nil
}

// ==========================================
// 4. 互亿无线短信通道
// ==========================================
type IhuyiSender struct {
	APIID      string
	APIKEY     string
	TemplateID string
}

func (i *IhuyiSender) SendSMS(phone, code, signName string) error {
	if i.APIID == "" || i.APIKEY == "" {
		return fmt.Errorf("互亿无线短信参数未配置")
	}

	var content string
	v := url.Values{}
	v.Set("account", i.APIID)

	if i.TemplateID != "" {
		content = code
		v.Set("templateid", i.TemplateID)
	} else {
		// 完整内容发送方式
		content = fmt.Sprintf("您的验证码是：%s。请不要把验证码泄露给其他人。", code)
	}

	timestamp := time.Now().Unix()
	timestampStr := fmt.Sprintf("%d", timestamp)

	// 动态密码公式: md5(account + APIKEY + mobile + content + time)
	hasher := md5.New()
	hasher.Write([]byte(i.APIID + i.APIKEY + phone + content + timestampStr))
	md5Password := hex.EncodeToString(hasher.Sum(nil))

	v.Set("password", md5Password)
	v.Set("mobile", phone)
	v.Set("content", content)
	v.Set("time", timestampStr)

	body := strings.NewReader(v.Encode())

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", "https://api.ihuyi.com/sms/Submit.json", body)
	if err != nil {
		return fmt.Errorf("创建互亿无线请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求互亿无线短信服务失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取互亿无线响应数据失败: %v", err)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析互亿无线响应失败: %v, body: %s", err, string(respBody))
	}

	if result.Code != 2 {
		return fmt.Errorf("互亿无线短信发送失败: [%d] %s", result.Code, result.Msg)
	}

	return nil
}

func (i *IhuyiSender) QueryBalance() (string, error) {
	if i.APIID == "" || i.APIKEY == "" {
		return "", fmt.Errorf("互亿无线短信参数未配置")
	}

	v := url.Values{}
	v.Set("account", i.APIID)
	v.Set("password", i.APIKEY)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("https://api.ihuyi.com/sms/GetNum.json", v)
	if err != nil {
		return "", fmt.Errorf("请求互亿无线余额接口失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取互亿无线余额响应失败: %v", err)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Num  int    `json:"num"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析互亿无线余额响应失败: %v, body: %s", err, string(respBody))
	}

	if result.Code != 2 {
		return "", fmt.Errorf("查询互亿无线余额失败: [%d] %s", result.Code, result.Msg)
	}

	return fmt.Sprintf("%d", result.Num), nil
}

// ==========================================
// 5. 短信精灵通道 (282930.cn)
// ==========================================
type SmsJinglingSender struct {
	UserID   string
	Username string
	Password string
}

func (s *SmsJinglingSender) SendSMS(phone, code, signName string) error {
	if s.Username == "" || s.Password == "" {
		return fmt.Errorf("短信精灵参数配置不完整")
	}

	v := url.Values{}
	v.Set("username", s.Username)
	v.Set("password", s.Password)
	v.Set("mobiles", phone)
	v.Set("content", fmt.Sprintf("【企业精灵】您的验证码是：%s", code))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("http://www.282930.cn/SMSReceiver.aspx", v)
	if err != nil {
		return fmt.Errorf("请求短信精灵发送接口失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取短信精灵响应数据失败: %v", err)
	}

	resultStr := strings.TrimSpace(string(respBody))
	if !strings.HasPrefix(resultStr, "0") {
		return fmt.Errorf("短信精灵发送失败，返回代码: %s", resultStr)
	}

	return nil
}

func (s *SmsJinglingSender) QueryBalance() (string, error) {
	if s.Username == "" || s.Password == "" {
		return "", fmt.Errorf("短信精灵配置不完整")
	}

	v := url.Values{}
	v.Set("username", s.Username)
	v.Set("password", s.Password)
	v.Set("queryoddcount", "1")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("http://www.282930.cn/SMSReceiver.aspx", v)
	if err != nil {
		return "", fmt.Errorf("请求短信精灵查询余额失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取短信精灵余额响应数据失败: %v", err)
	}

	return strings.TrimSpace(string(respBody)), nil
}

// ==========================================
// 双层计费扣减与加权选择核心逻辑
// ==========================================

// SendSMSAndBill 核心酒店层面扣费及高可用加权发送控制引擎
func SendSMSAndBill(ctx context.Context, hotelId int32, phone, ip, code string) error {
	// 1. 获取酒店信息
	hotel, err := GetHotelByHotelID(ctx, hotelId)
	if err != nil {
		return fmt.Errorf("查询酒店失败: %v", err)
	}
	if hotel == nil {
		return fmt.Errorf("该酒店 [ID:%d] 未注册或已被停用", hotelId)
	}
	if hotel.Status == 0 {
		return fmt.Errorf("该酒店已在平台禁用，无法提供验证服务")
	}

	ownerUser := hotel.User
	hotelsColl := MongoDB.Collection("hotels")

	// 2. 酒店层面可用库存原子扣减判定
	// 仅当酒店拥有的 sms_instock > 0 时，原子减 1
	res, err := hotelsColl.UpdateOne(ctx,
		bson.M{"hotelId": hotelId, "sms_instock": bson.M{"$gt": 0}},
		bson.M{"$inc": bson.M{"sms_instock": -1}},
	)
	if err != nil {
		return fmt.Errorf("扣除酒店短信库存数据库故障: %v", err)
	}
	if res.ModifiedCount == 0 {
		return fmt.Errorf("当前酒店短信发送配额已耗尽，上网认证受限，请提醒商家分拨配额")
	}

	billingType := "hotel_package" // 酒店配额划扣
	var deductedCount int32 = 1
	var deductedAmount int64 = 0

	// 3. 从数据库动态加载配置好的短信通道池进行加权分流
	provider, sender, err := selectSMSProvider(ctx)
	if err != nil {
		// 回滚酒店层面的库存
		rollbackHotelBilling(ctx, hotelId)
		return fmt.Errorf("短信平台无可用的发送通道: %v", err)
	}

	// 4. 调用所选的通道进行实际投递
	err = sender.SendSMS(phone, code, "") // 留空将自动降级使用短信通道配置的预设签名，避免 WelcomeText 冲突
	if err != nil {
		// 发送失败，执行回滚退款
		rollbackHotelBilling(ctx, hotelId)
		log.Printf("[%s] ❌ 短信发送失败: [酒店ID: %d] 手机号: %s, 通道: %s, 验证码: %s, 扣费类型: %s, 错误: %v",
			time.Now().Format("2006-01-02 15:04:05"), hotelId, phone, provider, code, billingType, err)
		return fmt.Errorf("短信发送投递失败: %v", err)
	}

	// 5. 发送成功：记录短信日志并存储至 Redis 缓存中
	SaveSMSLog(ctx, hotelId, phone, ip, code, billingType, deductedCount, deductedAmount, provider, ownerUser)
	log.Printf("[%s] 🚀 短信发送成功: [酒店ID: %d] 手机号: %s, 通道: %s, 验证码: %s, 扣费类型: %s",
		time.Now().Format("2006-01-02 15:04:05"), hotelId, phone, provider, code, billingType)

	resolvedCfg := ResolveHotelSecurityConfig(hotel, &GlobalConfig.Security)
	err = SaveCode(ctx, phone, ip, code, resolvedCfg)
	if err != nil {
		log.Printf("[%s] ⚠️ 验证码存储至 Redis 失败: %v", time.Now().Format("2006-01-02 15:04:05"), err)
	}

	return nil
}

// 针对酒店的库存退回回滚
func rollbackHotelBilling(ctx context.Context, hotelId int32) {
	hotelsColl := MongoDB.Collection("hotels")
	hotelsColl.UpdateOne(ctx, bson.M{"hotelId": hotelId}, bson.M{"$inc": bson.M{"sms_instock": 1}})
	log.Printf("↩️ 短信发送失败，已原子退回 1 条可用配额至酒店 %d 的库存", hotelId)
}

// 加权随机选择路由通道算法
func selectSMSProvider(ctx context.Context) (string, SMSSender, error) {
	providersColl := MongoDB.Collection("sms_providers")
	cursor, err := providersColl.Find(ctx, bson.M{"status": 1})
	if err != nil {
		return "", nil, err
	}
	defer cursor.Close(ctx)

	var list []SMSProvider
	if err = cursor.All(ctx, &list); err != nil {
		return "", nil, err
	}

	var activeList []SMSProvider
	var totalWeight int32 = 0
	for _, p := range list {
		if p.Weight > 0 {
			activeList = append(activeList, p)
			totalWeight += p.Weight
		}
	}

	if len(activeList) == 0 {
		return "", nil, fmt.Errorf("未找到任何启用的短信路由通道")
	}

	// 加权随机计算
	rand.Seed(time.Now().UnixNano())
	r := rand.Int31n(totalWeight)
	var cumulativeWeight int32 = 0
	var chosen SMSProvider

	for _, p := range activeList {
		cumulativeWeight += p.Weight
		if r < cumulativeWeight {
			chosen = p
			break
		}
	}

	// 实例化对应的发送接口
	var sender SMSSender
	switch chosen.Provider {
	case "aliyun":
		sender = &AliyunSender{
			AccessKeyID:     chosen.Config["access_key_id"],
			AccessKeySecret: chosen.Config["access_key_secret"],
			SignName:        chosen.Config["sign_name"],
			TemplateCode:    chosen.Config["template_code"],
		}
	case "tencent":
		sender = &TencentSender{
			SecretID:   chosen.Config["secret_id"],
			SecretKey:  chosen.Config["secret_key"],
			SDKAppID:   chosen.Config["sdk_app_id"],
			SignName:   chosen.Config["sign_name"],
			TemplateID: chosen.Config["template_id"],
		}
	case "ihuyi":
		sender = &IhuyiSender{
			APIID:      chosen.Config["api_id"],
			APIKEY:     chosen.Config["api_key"],
			TemplateID: chosen.Config["template_id"],
		}
	case "sms_jingling":
		sender = &SmsJinglingSender{
			UserID:   chosen.Config["userid"],
			Username: chosen.Config["username"],
			Password: chosen.Config["password"],
		}
	default:
		sender = &MockSender{
			signName: chosen.Config["sign_name"],
		}
	}

	return chosen.Provider, sender, nil
}

// ==========================================
// 辅助工具方法
// ==========================================

func sortKeys(keys []string) {
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
}

func specialUrlEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// GenerateRandomCode 生成指定位数的随机数字验证码
func GenerateRandomCode(length int) string {
	rand.Seed(time.Now().UnixNano())
	digits := "0123456789"
	var sb strings.Builder
	for i := 0; i < length; i++ {
		sb.WriteByte(digits[rand.Intn(len(digits))])
	}
	return sb.String()
}
