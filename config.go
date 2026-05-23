package main

import (
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
