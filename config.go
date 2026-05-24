package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port     int            `yaml:"port"`
	MongoDB  MongoDBConfig  `yaml:"mongodb"`
	Redis    RedisConfig    `yaml:"redis"`
	SMS      SMSConfig      `yaml:"sms"`
	Security SecurityConfig `yaml:"security"`
}

type MongoDBConfig struct {
	URI    string `yaml:"uri"`
	DBName string `yaml:"db_name"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type SMSConfig struct {
	PricePerSMS int `yaml:"price_per_sms"`
}

type SecurityConfig struct {
	SMSCooldown       int `yaml:"sms_cooldown"`
	IPCooldown        int `yaml:"ip_cooldown"`
	MaxSendsPerDay    int `yaml:"max_sends_per_day"`
	CodeExpireMinutes int `yaml:"code_expire_minutes"`
	MaxAttempts       int `yaml:"max_attempts"`
}

var GlobalConfig *Config

func LoadConfig(filePath string) (*Config, error) {
	// If filePath is config.yaml and it does not exist, write the default configuration
	if filePath == "config.yaml" {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Println("⚠️ 检测到缺失 config.yaml，正在以 tools/ikuai-portal/config.yaml 为参考自动生成默认配置文件...")
			defaultConfigContent := `# iKuai/Generic WiFi Captive Portal SaaS Server Configuration

# 服务器监听端口
port: 8080

# MongoDB 数据库配置
mongodb:
  uri: "mongodb://wifi:PHeewwjYyWw43ZB5@10.10.10.230:26110/wifi"
  db_name: "wifi"

# Redis 缓存配置 (防刷限流与验证码、会话存储)
redis:
  addr: "10.10.10.230:6379"
  password: "269898"
  db: 0

# 短信计费规则配置
sms:
  # 默认单条扣费价格 (单位: 分。此处 6 代表 0.06 元/条)
  price_per_sms: 6

# 安全与频率限制配置 (注: 酒店网关的冷却和上限均已改用数据库动态配置，此处仅作为全局默认备份)
security:
  # 单个手机号发送短信的冷却时间 (秒)
  sms_cooldown: 60
  # 单个 IP 发送短信的冷却时间 (秒)
  ip_cooldown: 60
  # 每个手机号/IP 每日最大允许发送短信次数 (0代表不限制)
  max_sends_per_day: 5
  # 验证码有效时长 (分钟，仍为全局配置)
  code_expire_minutes: 5
  # 验证码最大尝试匹配失败次数 (超过后验证码失效)
  max_attempts: 3
`
			if errWrite := os.WriteFile(filePath, []byte(defaultConfigContent), 0644); errWrite != nil {
				return nil, errWrite
			}
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Set default values if empty
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.MongoDB.URI == "" {
		cfg.MongoDB.URI = "mongodb://localhost:27017"
	}
	if cfg.MongoDB.DBName == "" {
		cfg.MongoDB.DBName = "wifi_saas"
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.SMS.PricePerSMS == 0 {
		cfg.SMS.PricePerSMS = 6
	}
	if cfg.Security.SMSCooldown == 0 {
		cfg.Security.SMSCooldown = 60
	}
	if cfg.Security.IPCooldown == 0 {
		cfg.Security.IPCooldown = 60
	}
	if cfg.Security.MaxSendsPerDay == 0 {
		cfg.Security.MaxSendsPerDay = 5
	}
	if cfg.Security.CodeExpireMinutes == 0 {
		cfg.Security.CodeExpireMinutes = 5
	}
	if cfg.Security.MaxAttempts == 0 {
		cfg.Security.MaxAttempts = 3
	}

	GlobalConfig = &cfg
	return &cfg, nil
}
