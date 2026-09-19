package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval      string            `yaml:"interval"`
	Concurrency   int               `yaml:"concurrency"`
	Once          bool              `yaml:"once"`
	FullPower     bool              `yaml:"full_power"`
	Timeout       int               `yaml:"timeout_seconds"`
	RatePerMinute int               `yaml:"rate_per_minute"`
	RatePerHour   int               `yaml:"rate_per_hour"`
	Headers       map[string]string `yaml:"headers"`
}

const defaultConfigPath = "config.yaml"

const configTemplate = `# ==========================================================================
# Nettap 并发请求工具配置文件
# 修改后重启程序生效
# ==========================================================================

# ------------------------- 执行模式 -------------------------

# 是否开启全力模式（true=不停发请求，压测到底，忽略 interval / rate / once）
full_power: false

# 执行间隔，支持的格式: 30s / 5m / 1h / 2h30m
# full_power: false 且 rate_per_minute / rate_per_hour 都没设时生效
interval: "1m"

# 是否只执行一轮后退出
# true=只跑一轮就退出，false=持续运行
once: false

# ------------------------- 速率控制（可选） -------------------------
# 设了 rate_per_minute 或 rate_per_hour，就按速率均匀发请求，忽略 interval
# 两个都设时以 rate_per_minute 为准
# 例: rate_per_minute: 30  => 每个接口每秒 0.5 次，2 秒发 1 次
# 例: rate_per_minute: 120 => 每个接口每秒 2 次
# 例: rate_per_hour: 3600  => 每个接口每秒 1 次（等同于 rate_per_minute: 60）
rate_per_minute: 0
rate_per_hour: 0

# ------------------------- 并发设置 -------------------------

# 每个接口的并发请求数
# 设为 0 或不填 = 自动计算
#   - 定时模式 / 单次执行: 自动 = 1
#   - 全力模式:           自动 = CPU核心数 × 2
#   - 速率模式:           自动 = 1
# 手动填 >0 就覆盖自动值
concurrency: 0

# ------------------------- 请求设置 -------------------------

# 单个请求的超时时间（秒），超时后该请求记为失败
timeout_seconds: 15

# ------------------------- 自定义请求头 -------------------------
# 需要额外加请求头时填在这里，已自动加了 Origin/Referer/User-Agent 等浏览器头
headers:
  # Cookie: "sessionid=xxxxx"
  # Authorization: "Bearer zzzzz"
`

func defaultConfig() Config {
	return Config{
		Interval:      "1m",
		Concurrency:   1,
		Once:          false,
		Timeout:       15,
		RatePerMinute: 0,
		RatePerHour:   0,
		Headers:       map[string]string{},
	}
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(configTemplate), 0644); err == nil {
				fmt.Printf("[nettap] 未找到 %s，已自动生成默认配置文件，请按需修改后重启\n", path)
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}

	if cfg.Interval == "" {
		cfg.Interval = "1m"
	}
	if cfg.Concurrency < 0 {
		cfg.Concurrency = 0
	}
	if cfg.Timeout < 1 {
		cfg.Timeout = 15
	}
	if cfg.RatePerMinute < 0 {
		cfg.RatePerMinute = 0
	}
	if cfg.RatePerHour < 0 {
		cfg.RatePerHour = 0
	}
	if cfg.RatePerMinute == 0 && cfg.RatePerHour > 0 {
		cfg.RatePerMinute = cfg.RatePerHour / 60
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}

	return cfg, nil
}
