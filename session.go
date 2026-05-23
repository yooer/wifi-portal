package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(ctx context.Context, addr, password string, db int) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	err := rdb.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("Redis 连接失败: %v", err)
	}

	RedisClient = rdb
	return nil
}

// -------------------------------------------------------------
// 短信验证码频次控制与安全校验 (Redis版)
// -------------------------------------------------------------

// ResolveHotelSecurityConfig 安全规则动态兜底算法
func ResolveHotelSecurityConfig(hotel *Hotel, global *SecurityConfig) *SecurityConfig {
	resolved := &SecurityConfig{
		SMSCooldown:       global.SMSCooldown,
		IPCooldown:        global.IPCooldown,
		MaxSendsPerDay:    global.MaxSendsPerDay,
		CodeExpireMinutes: global.CodeExpireMinutes,
		MaxAttempts:       global.MaxAttempts,
	}
	if hotel != nil {
		// 短信和 IP 冷却时间直接使用酒店配置数据中写入的值，不再回滚使用配置文件全局参数
		resolved.SMSCooldown = int(hotel.SMSCooldown)
		resolved.IPCooldown = int(hotel.IPCooldown)
		// 天发送上限使用酒店自定值（写入数据库；若为0则在校验处代表不限制）
		resolved.MaxSendsPerDay = int(hotel.MaxSendsDay)
	}
	return resolved
}

// AllowSend 检查是否允许发送短信 (包含冷却时间校验和每日防刷校验)
func AllowSend(ctx context.Context, phone, ip string, cfg *SecurityConfig) error {
	now := time.Now()
	todayStr := now.Format("20060102")

	// 1. 校验手机号发送冷却时间 (例如 60 秒)
	phoneCooldownKey := fmt.Sprintf("portal:sms:limit:phone:%s", phone)
	ttl, err := RedisClient.TTL(ctx, phoneCooldownKey).Result()
	if err == nil && ttl > 0 {
		return fmt.Errorf("该手机号获取验证码过于频繁，请在 %d 秒后再试", int(ttl.Seconds()))
	}

	// 2. 校验 IP 发送冷却时间
	ipCooldownKey := fmt.Sprintf("portal:sms:limit:ip:%s", ip)
	ttl, err = RedisClient.TTL(ctx, ipCooldownKey).Result()
	if err == nil && ttl > 0 {
		return fmt.Errorf("您的 IP 获取验证码过于频繁，请在 %d 秒后再试", int(ttl.Seconds()))
	}

	// 3. 校验手机号/IP 天级发送上限（如果限制次数 MaxSendsPerDay > 0 开启；如果为 0 代表不限制）
	if cfg.MaxSendsPerDay > 0 {
		phoneDailyKey := fmt.Sprintf("portal:sms:daily:phone:%s:%s", phone, todayStr)
		phoneSends, err := RedisClient.Get(ctx, phoneDailyKey).Int()
		if err == nil && phoneSends >= cfg.MaxSendsPerDay {
			return fmt.Errorf("该手机号今日发送验证码已达上限 (%d次)，请明日再试", cfg.MaxSendsPerDay)
		}

		// 4. 校验 IP 天级发送上限
		ipDailyKey := fmt.Sprintf("portal:sms:daily:ip:%s:%s", ip, todayStr)
		ipSends, err := RedisClient.Get(ctx, ipDailyKey).Int()
		if err == nil && ipSends >= cfg.MaxSendsPerDay {
			return fmt.Errorf("您当前 IP 今日发送验证码已达上限 (%d次)，请明日再试", cfg.MaxSendsPerDay)
		}
	}

	return nil
}

// SaveCode 缓存生成的验证码并建立限流冷却记录
func SaveCode(ctx context.Context, phone, ip, code string, cfg *SecurityConfig) error {
	now := time.Now()
	todayStr := now.Format("20060102")

	expireCode := time.Duration(cfg.CodeExpireMinutes) * time.Minute
	cooldownSMS := time.Duration(cfg.SMSCooldown) * time.Second
	cooldownIP := time.Duration(cfg.IPCooldown) * time.Second

	// 1. 缓存验证码及初始尝试次数
	codeKey := fmt.Sprintf("portal:sms:code:%s", phone)
	attemptsKey := fmt.Sprintf("portal:sms:attempts:%s", phone)

	err := RedisClient.Set(ctx, codeKey, code, expireCode).Err()
	if err != nil {
		return err
	}
	err = RedisClient.Set(ctx, attemptsKey, 0, expireCode).Err()
	if err != nil {
		return err
	}

	// 2. 建立发送限流冷却标识 (值存为1即可)
	phoneCooldownKey := fmt.Sprintf("portal:sms:limit:phone:%s", phone)
	RedisClient.Set(ctx, phoneCooldownKey, 1, cooldownSMS)

	ipCooldownKey := fmt.Sprintf("portal:sms:limit:ip:%s", ip)
	RedisClient.Set(ctx, ipCooldownKey, 1, cooldownIP)

	// 3. 增加手机号及 IP 日常总发送计数 (天级 Key)
	phoneDailyKey := fmt.Sprintf("portal:sms:daily:phone:%s:%s", phone, todayStr)
	RedisClient.Incr(ctx, phoneDailyKey)
	RedisClient.Expire(ctx, phoneDailyKey, 24*time.Hour) // 缓存一天即可

	ipDailyKey := fmt.Sprintf("portal:sms:daily:ip:%s:%s", ip, todayStr)
	RedisClient.Incr(ctx, ipDailyKey)
	RedisClient.Expire(ctx, ipDailyKey, 24*time.Hour)

	return nil
}

// VerifyCode 校验验证码 (OTP 机制)
func VerifyCode(ctx context.Context, phone, inputCode string, cfg *SecurityConfig) error {
	codeKey := fmt.Sprintf("portal:sms:code:%s", phone)
	attemptsKey := fmt.Sprintf("portal:sms:attempts:%s", phone)

	// 1. 查询验证码是否存在
	cachedCode, err := RedisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return fmt.Errorf("验证码未发送或已失效，请重新获取")
	} else if err != nil {
		return fmt.Errorf("校验失败，请重试")
	}

	// 2. 校验尝试错误次数
	attempts, err := RedisClient.Get(ctx, attemptsKey).Int()
	if err != nil {
		attempts = 0
	}
	if attempts >= cfg.MaxAttempts {
		RedisClient.Del(ctx, codeKey, attemptsKey) // 立即销毁
		return fmt.Errorf("验证码错误尝试次数过多，已失效，请重新获取")
	}

	// 3. 校验匹配值
	if cachedCode != inputCode {
		// 增加一次错误尝试
		newAttempts, _ := RedisClient.Incr(ctx, attemptsKey).Result()
		remaining := cfg.MaxAttempts - int(newAttempts)
		if remaining <= 0 {
			RedisClient.Del(ctx, codeKey, attemptsKey)
			return fmt.Errorf("验证码错误且尝试次数超限，已失效，请重新获取")
		}
		return fmt.Errorf("验证码错误，还可以尝试 %d 次", remaining)
	}

	// 4. 验证成功，删除验证码实现一次性校验安全
	RedisClient.Del(ctx, codeKey, attemptsKey)
	return nil
}

// -------------------------------------------------------------
// 商户会话 Session 缓存管理器
// -------------------------------------------------------------

type SessionUser struct {
	User  int64 `json:"user"`
	Level int32 `json:"level"`
}

// CreateSession 创建后台商户登录态
func CreateSession(ctx context.Context, token string, user int64, level int32) error {
	sessionKey := fmt.Sprintf("portal:session:%s", token)
	sUser := SessionUser{
		User:  user,
		Level: level,
	}
	data, err := json.Marshal(sUser)
	if err != nil {
		return err
	}
	return RedisClient.Set(ctx, sessionKey, string(data), 24*time.Hour).Err()
}

// GetSession 获取并验证后台商户登录态
func GetSession(ctx context.Context, token string) (*SessionUser, error) {
	sessionKey := fmt.Sprintf("portal:session:%s", token)
	data, err := RedisClient.Get(ctx, sessionKey).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var sUser SessionUser
	err = json.Unmarshal([]byte(data), &sUser)
	if err != nil {
		return nil, err
	}

	// 每次操作活跃时自动刷新 TTL 延长登录状态
	RedisClient.Expire(ctx, sessionKey, 24*time.Hour)
	return &sUser, nil
}

// DeleteSession 销毁后台登录态
func DeleteSession(ctx context.Context, token string) error {
	sessionKey := fmt.Sprintf("portal:session:%s", token)
	return RedisClient.Del(ctx, sessionKey).Err()
}

// DeleteAllSessionsForUser 销毁指定用户的所有活跃登录会话
func DeleteAllSessionsForUser(ctx context.Context, user int64) error {
	var cursor uint64
	for {
		keys, nextCursor, err := RedisClient.Scan(ctx, cursor, "portal:session:*", 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			val, err := RedisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var sUser SessionUser
			if err := json.Unmarshal([]byte(val), &sUser); err == nil {
				if sUser.User == user {
					RedisClient.Del(ctx, key)
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

