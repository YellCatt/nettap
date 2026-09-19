# Nettap 并发请求工具

定期并发请求两个接口，验证接口可用性和响应延迟。

## 接口
1. **首页统计**
   - GET https://api.insigmind.com/Api/WebLogin/GetIndexStats

2. **需求列表**
   - POST https://api.insigmind.com/Api/WebDemand/SearchDemands

## 构建 & 运行
```bash
go build -o nettap.exe .
./nettap.exe          # 按 config.yaml 运行
```

没有 config.yaml 时程序会自动生成一份默认的。

## 配置文件 config.yaml
改完重启生效：

```yaml
# 全力模式（true=不停发请求，压测到底，忽略 interval 和 once）
full_power: false

# 执行间隔，格式: 30s / 5m / 1h / 2h30m  (full_power: false 时生效)
interval: "1h"

# 是否只执行一轮后退出
once: false

# 每个接口的并发请求数
concurrency: 1

# 单个请求超时秒数
timeout_seconds: 15
```

### 三种模式对比

| 模式 | full_power | once | interval | 效果 |
|------|------------|------|----------|------|
| 定时执行 | `false` | `false` | `1h` | 每小时一轮，持续运行 |
| 单次执行 | `false` | `true` | - | 只跑一轮就退出 |
| 全力模式 | `true` | - | - | 两个接口不停发请求，每 5 秒打印一次 RPS/延迟统计 |

## 输出示例

**定时模式**
```
2026/09/19 10:04:39 [nettap] 启动 (version=dev)
2026/09/19 10:04:39 配置文件: config.yaml
2026/09/19 10:04:39 执行模式: 每 1h0m0s 一轮, 每接口并发 1
2026/09/19 10:04:39 ========== 新一轮开始 ==========
2026/09/19 10:04:39   ✓ 首页统计#1 | HTTP 200 | 耗时: 140ms | 响应: 1523 字节
2026/09/19 10:04:39   ⚠ 需求列表#1 | HTTP 403 | 耗时: 152ms | 响应: 199 字节
2026/09/19 10:04:39 ========== 本轮完成: 共 2 请求, 成功 1, 失败 1 ==========
```

**全力模式**
```
2026/09/19 10:15:00 [nettap] 启动 (version=dev)
2026/09/19 10:15:00 配置文件: config.yaml
2026/09/19 10:15:00 ⚡ 全力模式 | 每接口并发 10
⚡ [运行 5s] 总请求=842 成功=840 失败=2 | RPS=168.4/s | 平均=59ms 最小=12ms 最大=312ms
⚡ [运行 10s] 总请求=1685 成功=1683 失败=2 | RPS=168.6/s | 平均=59ms 最小=11ms 最大=308ms
```

状态标记：
- `✓` HTTP 200
- `⚠` HTTP 非 200
- `✗` 请求错误（网络超时、DNS 失败等）

## 时区
所有日志时间统一使用东八区（Asia/Shanghai / UTC+8）。