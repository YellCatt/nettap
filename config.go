package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval    string `yaml:"interval"`
	Concurrency int    `yaml:"concurrency"`
	Once        bool   `yaml:"once"`
	FullPower   bool   `yaml:"full_power"`
	Timeout     int    `yaml:"timeout_seconds"`
}

const defaultConfigPath = "config.yaml"

const configTemplate = `# ==========================================================================
# Nettap 并发请求工具配置文件
# 修改后重启程序生效，也可通过 -config=path/to/config.yaml 指定路径
# ==========================================================================

interval: "1h"            # 执行间隔，格式: 30s / 5m / 1h / 2h30m
once: false               # true=只跑一轮退出，false=持续运行
concurrency: 1            # 每个接口的并发请求数
full_power: false         # true=全力模式，不停发请求，忽略 interval
timeout_seconds: 15       # 单个请求超时秒数
`

func defaultConfig() Config {
	return Config{
		Interval:    "1h",
		Concurrency: 1,
		Once:        false,
		Timeout:     15,
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
		cfg.Interval = "1h"
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.Timeout < 1 {
		cfg.Timeout = 15
	}

	return cfg, nil
}
