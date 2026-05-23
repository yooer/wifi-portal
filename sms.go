package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
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
// 双层计费扣减与加权选择核心逻辑
// ==========================================

// SendSMSAndBill 核心双层扣费及高可用加权发送控制引擎
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
	usersColl := MongoDB.Collection("users")

	// 2. 双层原子扣费判定
	billingType := "free"
	var deductedCount int32 = 0
	var deductedAmount int64 = 0

	// 优先尝试原子扣减套餐 sms_count
	res, err := usersColl.UpdateOne(ctx,
		bson.M{"user": ownerUser, "sms_count": bson.M{"$gt": 0}},
		bson.M{"$inc": bson.M{"sms_count": -1}},
	)
	if err == nil && res.ModifiedCount > 0 {
		billingType = "package"
		deductedCount = 1
	} else {
		// 套餐不足，尝试从余额 balance 中扣除 (单位: 分。单条单价配置在全局配置中)
		price := int64(GlobalConfig.SMS.PricePerSMS)
		res, err = usersColl.UpdateOne(ctx,
			bson.M{"user": ownerUser, "balance": bson.M{"$gte": price}},
			bson.M{"$inc": bson.M{"balance": -price}},
		)
		if err != nil || res.ModifiedCount == 0 {
			// 两者都扣减失败，判定商户欠费
			return fmt.Errorf("商户短信余额不足，验证码发送受限，请提醒商家充值")
		}
		billingType = "balance"
		deductedAmount = price
		deductedCount = 1
	}

	// 3. 从数据库动态加载配置好的短信通道池进行加权分流
	provider, sender, err := selectSMSProvider(ctx)
	if err != nil {
		// 回滚扣费事务
		rollbackBilling(ctx, ownerUser, billingType, deductedAmount)
		return fmt.Errorf("短信平台无可用的发送通道: %v", err)
	}

	// 4. 调用所选的通道进行实际投递
	err = sender.SendSMS(phone, code, "") // 留空将自动降级使用短信通道配置的预设签名，避免 WelcomeText 冲突
	if err != nil {
		// 发送失败，执行回滚退款
		rollbackBilling(ctx, ownerUser, billingType, deductedAmount)
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

// 扣费事务回滚退款机制
func rollbackBilling(ctx context.Context, user int64, billingType string, amount int64) {
	usersColl := MongoDB.Collection("users")
	if billingType == "package" {
		usersColl.UpdateOne(ctx, bson.M{"user": user}, bson.M{"$inc": bson.M{"sms_count": 1}})
		log.Printf("↩️ 短信发送失败，已原子退回 1 条套餐额度给商户 %d", user)
	} else if billingType == "balance" {
		usersColl.UpdateOne(ctx, bson.M{"user": user}, bson.M{"$inc": bson.M{"balance": amount}})
		log.Printf("↩️ 短信发送失败，已原子退回 %.2f 元余额给商户 %d", float64(amount)/100.0, user)
	}
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
