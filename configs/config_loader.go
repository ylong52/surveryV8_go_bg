package configs

// 配置读取器，提供 Config.Get() 全局调用

import (
	"encoding/json"
	"os"
)

// Config 全局配置结构（代理、超时、路径等）
type Config struct {
	Proxy          string `json:"proxy"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	StoragePath    string `json:"storage_path"`
	SurveyType     string `json:"survey_type"`
}

var global *Config

// Load 从 configs/settings.json 读取配置并缓存到全局变量
func Load() *Config {
	data, err := os.ReadFile("configs/settings.json")
	if err != nil {
		global = &Config{TimeoutSeconds: 30, StoragePath: "./storage", SurveyType: "macalester_2026_prospective_families"}
		return global
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		global = &Config{TimeoutSeconds: 30, StoragePath: "./storage", SurveyType: "macalester_2026_prospective_families"}
		return global
	}
	global = cfg
	return cfg
}

// Get 返回已加载的全局配置，若未加载则先执行 Load
func Get() *Config {
	if global == nil {
		return Load()
	}
	return global
}
