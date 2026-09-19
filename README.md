# Nettap 并发请求工具

定期并发请求两个接口，验证接口可用性和响应延迟。

## 接口
1. **首页统计**
   - GET https://api.insigmind.com/Api/WebLogin/GetIndexStats

2. **需求列表**
   - POST https://api.insigmind.com/Api/WebDemand/SearchDemands

## 构建
```bash
go build -o nettap.exe .
```

## 配置文件 config.yaml
首次运行会在当前目录自动生成，手动修改后重启生效：

```yaml
# 执行间隔，支持的格式: 30s, 5m, 1h, 2h30m
interval: "1h"

# 每个接口的并发请求数
concurrency: 1

# 是否只执行一轮后退出（true=只跑一轮就退出，false=持续运行）
once: false

# 单个请求的超时时间（秒）
timeout_seconds: 15
```

## 命令行参数（优先级高于 config.yaml）
| 参数 | 说明 |
|------|------|
| `-config=config.yaml` | 指定配置文件路径 |
| `-interval=1m` | 覆盖配置文件中的间隔 |
| `-c 10` | 覆盖配置文件中的并发数 |
| `-once` | 覆盖为只跑一轮 |
| `-version` | 打印版本号 |

示例：
```bash
nettap.exe                                # 按 config.yaml 默认值运行
nettap.exe -once -interval=30s -c 10      # 临时跑一轮，每接口并发 10
nettap.exe -config=/path/to/config.yaml   # 指定其他配置文件
```

## 输出示例
```
2026/09/19 10:04:39 [nettap] 启动 (version=dev)
2026/09/19 10:04:39 配置文件: config.yaml
2026/09/19 10:04:39 执行模式: 每 1m0s 一轮, 每接口并发 3
2026/09/19 10:04:39 ========== 新一轮开始 ==========
2026/09/19 10:04:39 时间: 2026-09-19 10:04:39 | 并发数/接口: 3
2026/09/19 10:04:39   ⚠ 首页统计#1 | HTTP 403 | 耗时: 74ms | 响应: 199 字节
2026/09/19 10:04:39   ⚠ 需求列表#3 | HTTP 403 | 耗时: 76ms | 响应: 199 字节
2026/09/19 10:04:39 ========== 本轮完成: 共 6 请求, 成功 0, 失败 6 ==========
```

状态标记：
- `✓` HTTP 200
- `⚠` HTTP 非 200
- `✗` 请求错误（网络超时、DNS 失败等）

## 时区
所有日志时间统一使用东八区（Asia/Shanghai / UTC+8）。