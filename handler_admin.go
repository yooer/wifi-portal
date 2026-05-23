package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// crockfordBase32 字符集用于 ULID 编码
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateULID 纯 Go 实现的标准 Crockford Base32 排序 UUID
func GenerateULID() string {
	now := time.Now().UnixMilli()
	var tsPart [10]byte
	val := now
	for i := 9; i >= 0; i-- {
		tsPart[i] = crockfordBase32[val%32]
		val /= 32
	}

	var randPart [16]byte
	entropy := make([]byte, 10)
	_, _ = rand.Read(entropy)
	for i := 0; i < 16; i++ {
		idx := int(entropy[i%10]) % 32
		randPart[i] = crockfordBase32[idx]
	}

	return string(tsPart[:]) + string(randPart[:])
}

// -------------------------------------------------------------
// 1. 登录与会话管理
// -------------------------------------------------------------

type LoginRequest struct {
	User     int64  `json:"user"`
	Password string `json:"password"`
}

func HandleAdminLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"JSON 格式解析错误"}`))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 匹配用户
	usersColl := MongoDB.Collection("users")
	var u User
	err = usersColl.FindOne(ctx, bson.M{"user": req.User}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"该商户账号不存在"}`))
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"数据库查询失败"}`))
		return
	}

	// 校验密码
	if u.Password != HashPassword(req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"密码输入错误"}`))
		return
	}

	// 创建登录态加密 Session
	session := SessionPayload{
		User:  u.User,
		Level: u.Level,
	}
	err = GlobalAuth.CreateSession(w, &session, 86400) // 保持 24 小时
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"会话令牌生成失败"}`))
		return
	}

	// 写入 Redis Caching 激活 Session
	tokenCookie, _ := r.Cookie("portal_saas_session")
	if tokenCookie != nil {
		parts := strings.Split(tokenCookie.Value, ".")
		if len(parts) == 2 {
			CreateSession(ctx, parts[0], u.User, u.Level)
		}
	}

	respData := map[string]interface{}{
		"message": "登录成功",
		"user":    u.User,
		"level":   u.Level,
	}
	json.NewEncoder(w).Encode(respData)
}

func HandleAdminLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// 从 Redis 彻底注销
	tokenCookie, _ := r.Cookie("portal_saas_session")
	if tokenCookie != nil {
		parts := strings.Split(tokenCookie.Value, ".")
		if len(parts) == 2 {
			ctx := context.Background()
			DeleteSession(ctx, parts[0])
		}
	}

	GlobalAuth.DestroySession(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"注销成功"}`))
}

// =============================================================
// 1.5 酒店商户自主注册与短信发送 (验证码由系统垫付不计费)
// =============================================================

type RegisterSMSRequest struct {
	Phone string `json:"phone"`
}

func HandleAdminRegisterSendSMS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req RegisterSMSRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Phone == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"手机号参数解析错误"}`))
		return
	}

	phoneStr := req.Phone
	if len(phoneStr) != 11 || !strings.HasPrefix(phoneStr, "1") {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"请输入正确的中国11位手机号码"}`))
		return
	}

	phoneInt, err := strconv.ParseInt(phoneStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"非法手机号码"}`))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 检查该手机号是否已注册商户账户
	usersColl := MongoDB.Collection("users")
	var existingUser User
	err = usersColl.FindOne(ctx, bson.M{"user": phoneInt}).Decode(&existingUser)
	if err == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"该手机号码已注册商户账户，请直接登录"}`))
		return
	}

	// 2. Cooldown 冷却控制 (60秒冷却防短信轰炸)
	cooldownKey := fmt.Sprintf("portal:register:cooldown:%s", phoneStr)
	cdVal, _ := RedisClient.Get(ctx, cooldownKey).Result()
	if cdVal != "" {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"验证码发送过于频繁，请等待60秒后重试"}`))
		return
	}

	// 3. 产生6位验证码
	code := GenerateRandomCode(6)

	// 4. 选择通道进行发送 (系统公共通道免费投递)
	provider, sender, err := selectSMSProvider(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"系统短信发送通道不可用，请稍后再试"}`))
		return
	}

	err = sender.SendSMS(phoneStr, code, "")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"短信投递失败: %v"}`, err)))
		return
	}

	// 5. 存入 Redis 并设置5分钟有效期
	codeKey := fmt.Sprintf("portal:register:code:%s", phoneStr)
	RedisClient.Set(ctx, codeKey, code, 5*time.Minute)
	RedisClient.Set(ctx, cooldownKey, 1, 60*time.Second)

	log.Printf("[%s] 📢 自助注册验证码发送成功: 手机号 %s, 验证码 %s, 路由通道 %s",
		time.Now().Format("2006-01-02 15:04:05"), phoneStr, code, provider)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"验证码已发送，请注意查收"}`))
}

type RegisterRequest struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

func HandleAdminRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"仅支持 POST 请求"}`))
		return
	}

	var req RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Phone == "" || req.Code == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"请求参数不完整"}`))
		return
	}

	if len(req.Password) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"密码长度不能小于6位"}`))
		return
	}

	phoneStr := req.Phone
	phoneInt, err := strconv.ParseInt(phoneStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"非法手机号码"}`))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 再次确信手机号未被注册
	usersColl := MongoDB.Collection("users")
	var existingUser User
	err = usersColl.FindOne(ctx, bson.M{"user": phoneInt}).Decode(&existingUser)
	if err == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"该手机号码已注册商户账户，请直接登录"}`))
		return
	}

	// 2. 匹配 Redis 验证码
	codeKey := fmt.Sprintf("portal:register:code:%s", phoneStr)
	cachedCode, err := RedisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"验证码已过期或未获取，请重新获取"}`))
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"验证码校验服务异常，请重试"}`))
		return
	}

	if cachedCode != req.Code {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"验证码输入错误"}`))
		return
	}

	// 3. 校验通过，删除验证码防重用
	RedisClient.Del(ctx, codeKey)

	// 4. 新建酒店会员/商户 (余额为0，短信条数为0，酒店商户 level = 10)
	newUser := User{
		User:      phoneInt,
		Password:  HashPassword(req.Password),
		Level:     10, // 酒店商户
		Balance:   0,  // 余额 0
		SMSCount:  0,  // 短信条数 0
		CreatedAt: time.Now(),
	}

	_, err = usersColl.InsertOne(ctx, newUser)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"注册失败，请稍后重试"}`))
		return
	}

	// 5. 自动完成登录，省去用户二次登录步骤
	session := SessionPayload{
		User:  newUser.User,
		Level: newUser.Level,
	}
	err = GlobalAuth.CreateSession(w, &session, 86400)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"注册成功，但自动建立登录会话失败，请手动登录"}`))
		return
	}

	// 注入 Redis Caching 会话
	tokenCookie, _ := r.Cookie("portal_saas_session")
	if tokenCookie != nil {
		parts := strings.Split(tokenCookie.Value, ".")
		if len(parts) == 2 {
			CreateSession(ctx, parts[0], newUser.User, newUser.Level)
		}
	}

	respData := map[string]interface{}{
		"message": "注册成功并已自动登录",
		"user":    newUser.User,
		"level":   newUser.Level,
	}
	json.NewEncoder(w).Encode(respData)
}

func HandleAdminProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取中间件 Header
	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var u User
	err := MongoDB.Collection("users").FindOne(ctx, bson.M{"user": userAccount}).Decode(&u)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"未找到对应账号"}`))
		return
	}

	json.NewEncoder(w).Encode(u)
}

// -------------------------------------------------------------
// 2. 超级管理员专享 APIs (RequireAdmin)
// -------------------------------------------------------------

// HandleSuperUsers 处理商户账号增删改查
func HandleSuperUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	usersColl := MongoDB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		// 查询商户列表
		cursor, err := usersColl.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, `{"error":"查询失败"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var list []User
		cursor.All(ctx, &list)
		for i := range list {
			list[i].Password = "" // 隐藏哈希密文，防窃听
		}
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		// 创建商户/管理员
		var input User
		json.NewDecoder(r.Body).Decode(&input)
		if input.User == 0 || input.Password == "" {
			http.Error(w, `{"error":"缺少账号或密码"}`, http.StatusBadRequest)
			return
		}

		input.Password = HashPassword(input.Password)
		if input.Level == 0 {
			input.Level = 10 // 默认商户
		}
		input.CreatedAt = time.Now()

		_, err := usersColl.InsertOne(ctx, input)
		if err != nil {
			http.Error(w, `{"error":"该商户账号已注册"}`, http.StatusConflict)
			return
		}
		json.NewEncoder(w).Encode(input)

	case http.MethodPut:
		// 更新商户数据 (包含重置密码)
		var input map[string]interface{}
		json.NewDecoder(r.Body).Decode(&input)
		userVal, ok := input["user"]
		if !ok {
			http.Error(w, `{"error":"缺少商户账号 user"}`, http.StatusBadRequest)
			return
		}

		var targetUser int64
		switch v := userVal.(type) {
		case float64:
			targetUser = int64(v)
		case string:
			targetUser, _ = strconv.ParseInt(v, 10, 64)
		}

		updateDoc := bson.M{}
		if lvl, exists := input["level"]; exists {
			updateDoc["level"] = int32(lvl.(float64))
		}
		if bal, exists := input["balance"]; exists {
			updateDoc["balance"] = int64(bal.(float64))
		}
		if sCount, exists := input["sms_count"]; exists {
			updateDoc["sms_count"] = int32(sCount.(float64))
		}
		if pwd, exists := input["password"]; exists && pwd.(string) != "" {
			updateDoc["password"] = HashPassword(pwd.(string))
		}

		if len(updateDoc) == 0 {
			http.Error(w, `{"error":"没有需要更新的字段"}`, http.StatusBadRequest)
			return
		}

		_, err := usersColl.UpdateOne(ctx, bson.M{"user": targetUser}, bson.M{"$set": updateDoc})
		if err != nil {
			http.Error(w, `{"error":"更新失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"更新商户资料成功"}`))

	case http.MethodDelete:
		// 删除商户及级联数据
		userStr := r.URL.Query().Get("user")
		if userStr == "" {
			http.Error(w, `{"error":"缺少商户账号 user"}`, http.StatusBadRequest)
			return
		}

		targetUser, err := strconv.ParseInt(userStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"非法商户账号"}`, http.StatusBadRequest)
			return
		}

		// 安全锁：禁止删除初始超管 13703770377
		if targetUser == 13703770377 {
			http.Error(w, `{"error":"初始超级管理员账号禁止删除"}`, http.StatusForbidden)
			return
		}

		// 1. 查出该商户旗下的所有酒店ID
		hotelsColl := MongoDB.Collection("hotels")
		cursor, err := hotelsColl.Find(ctx, bson.M{"user": targetUser})
		var hotelIDs []int32
		if err == nil {
			var hotels []Hotel
			cursor.All(ctx, &hotels)
			for _, h := range hotels {
				hotelIDs = append(hotelIDs, h.HotelID)
			}
			cursor.Close(ctx)
		}

		// 2. 物理级联删除旗下酒店的所有连网日志 auth_logs
		if len(hotelIDs) > 0 {
			MongoDB.Collection("auth_logs").DeleteMany(ctx, bson.M{"hotelId": bson.M{"$in": hotelIDs}})
		}

		// 3. 物理级联删除该商户的所有短信日志 sms_logs (基于 user 进行清理，支持无酒店时的彻底清理)
		MongoDB.Collection("sms_logs").DeleteMany(ctx, bson.M{"user": targetUser})

		// 4. 物理级联删除旗下所有酒店节点 hotels
		hotelsColl.DeleteMany(ctx, bson.M{"user": targetUser})

		// 5. 物理级联删除该商户的所有财务账单流水分期 recharge
		MongoDB.Collection("recharge").DeleteMany(ctx, bson.M{"user": targetUser})

		// 6. 销毁该商户在 Redis 中的所有活跃登录会话
		_ = DeleteAllSessionsForUser(ctx, targetUser)

		// 7. 最终物理删除该商户账号本身 users
		_, err = usersColl.DeleteOne(ctx, bson.M{"user": targetUser})
		if err != nil {
			http.Error(w, `{"error":"物理删除账户失败"}`, http.StatusInternalServerError)
			return
		}

		log.Printf("[%s] 🗑️ 超级管理员执行物理销户: 商户 %d 及其全部关联级联数据已彻底清理",
			time.Now().Format("2006-01-02 15:04:05"), targetUser)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"销户及级联数据清理成功"}`))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleSuperHotels 超级管理员对酒店的管理 (含自动 hotelId 序列生成)
func HandleSuperHotels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	hotelsColl := MongoDB.Collection("hotels")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		cursor, err := hotelsColl.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, `{"error":"查询失败"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var list []Hotel
		cursor.All(ctx, &list)
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var input Hotel
		json.NewDecoder(r.Body).Decode(&input)
		if input.Name == "" || input.User == 0 {
			http.Error(w, `{"error":"缺少酒店名或所属商户账号"}`, http.StatusBadRequest)
			return
		}

		// 原子自增生成全局唯一的整型 hotelId (从 1000000 开始)
		hId, err := GenerateNextHotelID(ctx)
		if err != nil {
			http.Error(w, `{"error":"生成自增 hotelId 失败: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		input.HotelID = hId
		if input.GatewayType == "" {
			input.GatewayType = "ikuai"
		}
		input.Status = 1
		input.CreatedAt = time.Now()

		_, err = hotelsColl.InsertOne(ctx, input)
		if err != nil {
			http.Error(w, `{"error":"酒店新建写入失败"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(input)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleSuperPackages 套餐配置集合 (Collection: packages) 管理 (增删改查)
func HandleSuperPackages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	packagesColl := MongoDB.Collection("packages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		cursor, err := packagesColl.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, `{"error":"获取套餐失败"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var list []Package
		cursor.All(ctx, &list)
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var p Package
		json.NewDecoder(r.Body).Decode(&p)
		if p.PackageID == "" || p.Name == "" || p.Price <= 0 || p.SMSCount <= 0 {
			http.Error(w, `{"error":"套餐表单输入项不完整"}`, http.StatusBadRequest)
			return
		}
		p.Status = 1
		p.CreatedAt = time.Now()
		_, err := packagesColl.InsertOne(ctx, p)
		if err != nil {
			http.Error(w, `{"error":"该套餐标识已存在"}`, http.StatusConflict)
			return
		}
		json.NewEncoder(w).Encode(p)

	case http.MethodPut:
		var p Package
		json.NewDecoder(r.Body).Decode(&p)
		_, err := packagesColl.UpdateOne(ctx, bson.M{"packageId": p.PackageID}, bson.M{"$set": bson.M{
			"name":      p.Name,
			"price":     p.Price,
			"sms_count": p.SMSCount,
			"status":    p.Status,
		}})
		if err != nil {
			http.Error(w, `{"error":"更新失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"套餐配置修改成功"}`))

	case http.MethodDelete:
		packageId := r.URL.Query().Get("packageId")
		if packageId == "" {
			http.Error(w, `{"error":"缺少 packageId"}`, http.StatusBadRequest)
			return
		}
		_, err := packagesColl.DeleteOne(ctx, bson.M{"packageId": packageId})
		if err != nil {
			http.Error(w, `{"error":"删除失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"套餐删除成功"}`))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleSuperSMSProviders 短信通道配置集合 (Collection: sms_providers) 管理 (增删改查)
func HandleSuperSMSProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	providersColl := MongoDB.Collection("sms_providers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		cursor, err := providersColl.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, `{"error":"获取短信通道失败"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var list []SMSProvider
		cursor.All(ctx, &list)
		if list == nil {
			list = []SMSProvider{}
		}

		// 为安全起见，遮罩 API Key/Secret
		for idx, item := range list {
			if item.Config != nil {
				maskedConfig := make(map[string]string)
				for k, v := range item.Config {
					if strings.Contains(strings.ToLower(k), "secret") || strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "passwd") || strings.Contains(strings.ToLower(k), "password") {
						if len(v) > 0 {
							maskedConfig[k] = "******"
						} else {
							maskedConfig[k] = ""
						}
					} else {
						maskedConfig[k] = v
					}
				}
				list[idx].Config = maskedConfig
			}
		}
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var p SMSProvider
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			http.Error(w, `{"error":"解析参数失败"}`, http.StatusBadRequest)
			return
		}
		if p.Provider == "" || p.Weight < 0 {
			http.Error(w, `{"error":"参数不完整"}`, http.StatusBadRequest)
			return
		}
		p.ID = primitive.NewObjectID()
		p.CreatedAt = time.Now()
		if p.Config == nil {
			p.Config = make(map[string]string)
		}
		_, err = providersColl.InsertOne(ctx, p)
		if err != nil {
			http.Error(w, `{"error":"保存短信通道失败"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(p)

	case http.MethodPut:
		var p SMSProvider
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			http.Error(w, `{"error":"解析参数失败"}`, http.StatusBadRequest)
			return
		}
		if p.ID.IsZero() {
			http.Error(w, `{"error":"缺少通道ID"}`, http.StatusBadRequest)
			return
		}
		
		// 如果上传的 config 中 secret/key 是 "******"，说明用户没有修改它，需要从数据库中读取并保留原值
		var existing SMSProvider
		err = providersColl.FindOne(ctx, bson.M{"_id": p.ID}).Decode(&existing)
		if err == nil && existing.Config != nil {
			if p.Config == nil {
				p.Config = make(map[string]string)
			}
			for k, v := range p.Config {
				if v == "******" {
					if origVal, exists := existing.Config[k]; exists {
						p.Config[k] = origVal
					}
				}
			}
		}

		if p.Config == nil {
			p.Config = make(map[string]string)
		}

		_, err = providersColl.UpdateOne(ctx, bson.M{"_id": p.ID}, bson.M{"$set": bson.M{
			"provider": p.Provider,
			"weight":   p.Weight,
			"config":   p.Config,
			"status":   p.Status,
		}})
		if err != nil {
			http.Error(w, `{"error":"更新通道失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"更新短信通道成功"}`))

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, `{"error":"缺少通道ID"}`, http.StatusBadRequest)
			return
		}
		objID, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, `{"error":"通道ID格式错误"}`, http.StatusBadRequest)
			return
		}
		_, err = providersColl.DeleteOne(ctx, bson.M{"_id": objID})
		if err != nil {
			http.Error(w, `{"error":"删除通道失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"删除通道成功"}`))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleSuperRechargeManual 管理员手动充值余额/买套餐接口
func HandleSuperRechargeManual(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"仅支持 POST 请求"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		User        int64  `json:"user"`
		Type        string `json:"type"` // "balance" | "package"
		Amount      int64  `json:"amount"` // 分
		SMSCount    int32  `json:"sms_count"`
		PackageName string `json:"package_name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.User == 0 || req.Amount <= 0 {
		http.Error(w, `{"error":"参数格式有误"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 原子累加商户账户
	usersColl := MongoDB.Collection("users")
	updateDoc := bson.M{"$inc": bson.M{"balance": req.Amount}}
	if req.Type == "package" {
		updateDoc = bson.M{"$inc": bson.M{"balance": req.Amount, "sms_count": req.SMSCount}}
	}

	res, err := usersColl.UpdateOne(ctx, bson.M{"user": req.User}, updateDoc)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, `{"error":"充值失败，商户账号不存在"}`, http.StatusNotFound)
		return
	}

	// 自动生成 ULID orderId 写入充值记录
	orderId := GenerateULID()
	rechargeDoc := Recharge{
		OrderID:     orderId,
		User:        req.User,
		Type:        req.Type,
		Amount:      req.Amount,
		SMSCount:    req.SMSCount,
		PackageName: req.PackageName,
		CreatedAt:   time.Now(),
	}

	MongoDB.Collection("recharge").InsertOne(ctx, rechargeDoc)

	json.NewEncoder(w).Encode(rechargeDoc)
}

// -------------------------------------------------------------
// 3. 酒店商户专享 APIs (酒店管理者级过滤)
// -------------------------------------------------------------

// HandleMerchantHotels 商户对其关联酒店信息的个性化设置
func HandleMerchantHotels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)
	userLevelStr := r.Header.Get("X-User-Level")
	userLevel, _ := strconv.Atoi(userLevelStr)

	hotelsColl := MongoDB.Collection("hotels")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		// 商户查看自己名下的酒店列表
		cursor, err := hotelsColl.Find(ctx, bson.M{"user": userAccount})
		if err != nil {
			http.Error(w, `{"error":"查询失败"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var list []Hotel
		cursor.All(ctx, &list)
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		// 商户或超管创建新酒店节点
		var input Hotel
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, `{"error":"解析请求参数失败"}`, http.StatusBadRequest)
			return
		}
		if input.Name == "" {
			http.Error(w, `{"error":"缺少酒店名称"}`, http.StatusBadRequest)
			return
		}

		// 安全防护：普通商户创建新节点时，必须且只能将其强行归属为其自身账号
		if userLevel < 50 {
			input.User = userAccount
		} else {
			// 超级管理员创建时，若未指定归属商户，则默认绑定其自身账号
			if input.User == 0 {
				input.User = userAccount
			}
		}

		// 原子自增生成全局唯一的 7 位整型 hotelId (如 1000001)
		hId, err := GenerateNextHotelID(ctx)
		if err != nil {
			http.Error(w, `{"error":"生成自增 hotelId 失败: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		input.HotelID = hId
		if input.GatewayType == "" {
			input.GatewayType = "ikuai"
		}
		input.Status = 1
		input.CreatedAt = time.Now()

		_, err = hotelsColl.InsertOne(ctx, input)
		if err != nil {
			http.Error(w, `{"error":"创建酒店节点写入数据库失败"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(input)

	case http.MethodPut:
		// 商户修改自己酒店的配置
		var input Hotel
		json.NewDecoder(r.Body).Decode(&input)

		// 权限核查：该酒店是否真的归属该商户 (超级管理员免检)
		var existing Hotel
		var err error
		if userLevel >= 50 {
			err = hotelsColl.FindOne(ctx, bson.M{"hotelId": input.HotelID}).Decode(&existing)
		} else {
			err = hotelsColl.FindOne(ctx, bson.M{"hotelId": input.HotelID, "user": userAccount}).Decode(&existing)
		}
		if err != nil {
			http.Error(w, `{"error":"酒店不存在或您无权管理该酒店"}`, http.StatusForbidden)
			return
		}

		updateFields := bson.M{
			"name":          input.Name,
			"welcome_text":  input.WelcomeText,
			"gateway_type":  input.GatewayType,
			"custom_name":   input.CustomName,
			"app_key":       input.AppKey,
			"sms_cooldown":  input.SMSCooldown,
			"ip_cooldown":   input.IPCooldown,
			"max_sends_day": input.MaxSendsDay,
			"bypass_auth":   input.BypassAuth,
		}

		_, err = hotelsColl.UpdateOne(ctx, bson.M{"hotelId": input.HotelID}, bson.M{"$set": updateFields})
		if err != nil {
			http.Error(w, `{"error":"个性配置更新失败"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"message":"酒店属性修改成功"}`))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleMerchantBuyPackage 商户购买套餐扣费逻辑 (原子)
func HandleMerchantBuyPackage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"仅支持 POST 请求"}`, http.StatusMethodNotAllowed)
		return
	}

	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)

	var req struct {
		PackageID string `json:"packageId"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 查询套餐数据
	var pkg Package
	err := MongoDB.Collection("packages").FindOne(ctx, bson.M{"packageId": req.PackageID, "status": 1}).Decode(&pkg)
	if err != nil {
		http.Error(w, `{"error":"套餐包不存在或已被下架"}`, http.StatusBadRequest)
		return
	}

	// 2. 安全扣除商户余额并原子增加短信条数
	usersColl := MongoDB.Collection("users")
	res, err := usersColl.UpdateOne(ctx,
		bson.M{"user": userAccount, "balance": bson.M{"$gte": pkg.Price}},
		bson.M{"$inc": bson.M{"balance": -pkg.Price, "sms_count": pkg.SMSCount}},
	)
	if err != nil || res.ModifiedCount == 0 {
		http.Error(w, `{"error":"购买失败，您的账户余额不足，请联系管理员充值"}`, http.StatusPaymentRequired)
		return
	}

	// 3. 产生充值流水 (ULID orderId)
	orderId := GenerateULID()
	rechargeDoc := Recharge{
		OrderID:     orderId,
		User:        userAccount,
		Type:        "package",
		Amount:      pkg.Price,
		SMSCount:    pkg.SMSCount,
		PackageName: pkg.Name,
		CreatedAt:   time.Now(),
	}

	MongoDB.Collection("recharge").InsertOne(ctx, rechargeDoc)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "套餐包订购成功",
		"orderId":   orderId,
		"sms_count": pkg.SMSCount,
	})
}

// HandleMerchantAuthLogs 获取当前商户旗下酒店的所有连网记录 (如果是超级管理员，则返回全网最新200条记录)
func HandleMerchantAuthLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)
	userLevelStr := r.Header.Get("X-User-Level")
	userLevel, _ := strconv.Atoi(userLevelStr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 如果是超级管理员，免除酒店归属过滤，直接返回全表最新的 200 条记录
	if userLevel >= 50 {
		logCursor, err := MongoDB.Collection("auth_logs").Find(ctx,
			bson.M{},
			options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(200),
		)
		if err != nil {
			http.Error(w, `{"error":"查询全局上网日志失败"}`, http.StatusInternalServerError)
			return
		}
		defer logCursor.Close(ctx)

		var list []AuthLog
		if err := logCursor.All(ctx, &list); err != nil {
			http.Error(w, `{"error":"解析全局上网日志失败"}`, http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []AuthLog{}
		}
		json.NewEncoder(w).Encode(list)
		return
	}

	// 2. 普通商户：只提取自己名下的 hotelId
	cursor, err := MongoDB.Collection("hotels").Find(ctx, bson.M{"user": userAccount})
	if err != nil {
		http.Error(w, `{"error":"查询失败"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var hotels []Hotel
	cursor.All(ctx, &hotels)
	if len(hotels) == 0 {
		w.Write([]byte(`[]`))
		return
	}

	var hotelIDs []int32
	for _, h := range hotels {
		hotelIDs = append(hotelIDs, h.HotelID)
	}

	// 3. 秒级查询审计放行日志
	logCursor, err := MongoDB.Collection("auth_logs").Find(ctx,
		bson.M{"hotelId": bson.M{"$in": hotelIDs}},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(100),
	)
	if err != nil {
		http.Error(w, `{"error":"查询放行日志失败"}`, http.StatusInternalServerError)
		return
	}
	defer logCursor.Close(ctx)

	var list []AuthLog
	logCursor.All(ctx, &list)
	if list == nil {
		list = []AuthLog{}
	}
	json.NewEncoder(w).Encode(list)
}

// HandleMerchantSMSLogs 获取当前商户名下的所有短信详单明细 (如果是超级管理员，则返回全网最新200条详单)
func HandleMerchantSMSLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)
	userLevelStr := r.Header.Get("X-User-Level")
	userLevel, _ := strconv.Atoi(userLevelStr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 如果是超级管理员，免除酒店归属过滤，直接返回全表最新的 200 条记录
	if userLevel >= 50 {
		logCursor, err := MongoDB.Collection("sms_logs").Find(ctx,
			bson.M{},
			options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(200),
		)
		if err != nil {
			http.Error(w, `{"error":"查询全局短信日志失败"}`, http.StatusInternalServerError)
			return
		}
		defer logCursor.Close(ctx)

		var list []SMSLog
		if err := logCursor.All(ctx, &list); err != nil {
			http.Error(w, `{"error":"解析全局短信日志失败"}`, http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []SMSLog{}
		}
		json.NewEncoder(w).Encode(list)
		return
	}

	// 2. 普通商户：只提取自己名下的 hotelId
	cursor, err := MongoDB.Collection("hotels").Find(ctx, bson.M{"user": userAccount})
	if err != nil {
		http.Error(w, `{"error":"查询商户酒店失败"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var hotels []Hotel
	cursor.All(ctx, &hotels)
	if len(hotels) == 0 {
		w.Write([]byte(`[]`))
		return
	}

	var hotelIDs []int32
	for _, h := range hotels {
		hotelIDs = append(hotelIDs, h.HotelID)
	}

	// 3. 普通商户：查询自己酒店对应的短信日志
	logCursor, err := MongoDB.Collection("sms_logs").Find(ctx,
		bson.M{"hotelId": bson.M{"$in": hotelIDs}},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(100),
	)
	if err != nil {
		http.Error(w, `{"error":"查询短信日志失败"}`, http.StatusInternalServerError)
		return
	}
	defer logCursor.Close(ctx)

	var list []SMSLog
	logCursor.All(ctx, &list)
	if list == nil {
		list = []SMSLog{}
	}
	json.NewEncoder(w).Encode(list)
}

// HandleMerchantRechargeRecords 查询商户的充值记录明细
func HandleMerchantRechargeRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAccountStr := r.Header.Get("X-User-Account")
	userAccount, _ := strconv.ParseInt(userAccountStr, 10, 64)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logCursor, err := MongoDB.Collection("recharge").Find(ctx,
		bson.M{"user": userAccount},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(50),
	)
	if err != nil {
		http.Error(w, `{"error":"查询充值流水分支失败"}`, http.StatusInternalServerError)
		return
	}
	defer logCursor.Close(ctx)

	var list []Recharge
	logCursor.All(ctx, &list)
	json.NewEncoder(w).Encode(list)
}
