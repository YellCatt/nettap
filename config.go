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
	Timeout     int    `yaml:"timeout_seconds"`
}

const defaultConfigPath = "config.yaml"

const configTemplate = `# nettap 配置文件（首次运行自动生成，可按需修改，重启后生效）

# 执行间隔，支持的格式: 30s, 5m, 1h, 2h30m
interval: "1h"

# 每个接口的并发请求数
concurrency: 1

# 是否只执行一轮后退出（true=只跑一轮就退出，false=持续运行）
once: false

# 单个请求的超时时间（秒）
timeout_seconds: 15
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
