package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	statsAPIURL   = "https://api.insigmind.com/Api/WebLogin/GetIndexStats"
	demandAPIURL  = "https://api.insigmind.com/Api/WebDemand/SearchDemands"
	demandPayload = `{"pageSize":10,"total":0,"pageNumber":1,"demandUrban":"","demandExpirationDate":"","newDate":"","sortFieldName":"AuditProperties.CreateAt","sortDirection":"Descending","demandStatus":1}`
)

var version = "dev"

var chinaLoc *time.Location

var httpTimeout = 15 * time.Second

var customHeaders = map[string]string{}

func initChinaLoc() {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil && loc != nil {
		chinaLoc = loc
	} else {
		chinaLoc = time.FixedZone("CST", 8*60*60)
	}
}

func nowCST() time.Time {
	if chinaLoc == nil {
		initChinaLoc()
	}
	return time.Now().In(chinaLoc)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

type apiResult struct {
	name     string
	url      string
	status   int
	elapsed  time.Duration
	err      error
	bodySize int
	body     string
	success  bool
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func doRequest(name, url, method, body string) apiResult {
	client := newHTTPClient()
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return apiResult{name: name, url: url, err: err}
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", "https://www.insigmind.com")
	req.Header.Set("Referer", "https://www.insigmind.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, v := range customHeaders {
		req.Header.Set(k, v)
	}

	start := nowCST()
	resp, err := client.Do(req)
	if err != nil {
		return apiResult{name: name, url: url, elapsed: time.Since(start), err: err}
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return apiResult{
		name:     name,
		url:      url,
		status:   resp.StatusCode,
		elapsed:  time.Since(start),
		bodySize: len(data),
		body:     truncate(string(data), 200),
		success:  resp.StatusCode == 200,
	}
}

func runRound(concurrency int) {
	log.Printf("========== 新一轮开始 ==========")
	log.Printf("时间: %s | 并发数/接口: %d", nowCST().Format("2006-01-02 15:04:05"), concurrency)

	type task struct {
		name   string
		url    string
		method string
		body   string
	}

	tasks := []task{
		{"首页统计", statsAPIURL, "GET", ""},
		{"需求列表", demandAPIURL, "POST", demandPayload},
	}

	var wg sync.WaitGroup
	results := make(chan apiResult, len(tasks)*concurrency)

	for _, t := range tasks {
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(tk task, idx int) {
				defer wg.Done()
				r := doRequest(tk.name, tk.url, tk.method, tk.body)
				r.name = fmt.Sprintf("%s#%d", tk.name, idx+1)
				results <- r
			}(t, i)
		}
	}

	wg.Wait()
	close(results)

	total := 0
	success := 0
	fail := 0
	for r := range results {
		total++
		if r.err != nil {
			fail++
			log.Printf("  ✗ %s | 错误: %v | 耗时: %v", r.name, r.err, r.elapsed.Round(time.Millisecond))
		} else if !r.success {
			fail++
			log.Printf("  ⚠ %s | HTTP %d | 耗时: %v | 响应: %s", r.name, r.status, r.elapsed.Round(time.Millisecond), r.body)
		} else {
			success++
			log.Printf("  ✓ %s | HTTP %d | 耗时: %v | 响应: %d 字节", r.name, r.status, r.elapsed.Round(time.Millisecond), r.bodySize)
		}
	}
	log.Printf("========== 本轮完成: 共 %d 请求, 成功 %d, 失败 %d ==========", total, success, fail)
}

func runFullPower(concurrency int) {
	log.Printf("⚡ 全力模式启动！每接口并发 %d，不停发请求", concurrency)

	type task struct {
		name   string
		url    string
		method string
		body   string
	}

	tasks := []task{
		{"首页统计", statsAPIURL, "GET", ""},
		{"需求列表", demandAPIURL, "POST", demandPayload},
	}

	type stats struct {
		mu       sync.Mutex
		total    int
		success  int
		fail     int
		totalDur time.Duration
		minDur   time.Duration
		maxDur   time.Duration
		failBuf  []apiResult
	}

	s := &stats{}

	for _, t := range tasks {
		for i := 0; i < concurrency; i++ {
			go func(tk task) {
				for {
					r := doRequest(tk.name, tk.url, tk.method, tk.body)
					s.mu.Lock()
					s.total++
					s.totalDur += r.elapsed
					if s.minDur == 0 || r.elapsed < s.minDur {
						s.minDur = r.elapsed
					}
					if r.elapsed > s.maxDur {
						s.maxDur = r.elapsed
					}
					if r.err != nil || !r.success {
						s.fail++
						if len(s.failBuf) >= 5 {
							s.failBuf = s.failBuf[1:]
						}
						s.failBuf = append(s.failBuf, r)
					} else {
						s.success++
					}
					s.mu.Unlock()
				}
			}(t)
		}
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	startTime := nowCST()
	lastTotal := 0
	lastTime := startTime

	for range ticker.C {
		s.mu.Lock()
		total := s.total
		success := s.success
		fail := s.fail
		avgDur := time.Duration(0)
		if total > 0 {
			avgDur = s.totalDur / time.Duration(total)
		}
		minDur := s.minDur
		maxDur := s.maxDur
		failBuf := make([]apiResult, len(s.failBuf))
		copy(failBuf, s.failBuf)
		s.mu.Unlock()

		now := nowCST()
		elapsed := now.Sub(lastTime)
		intervalCount := total - lastTotal
		rps := float64(intervalCount) / elapsed.Seconds()
		lastTotal = total
		lastTime = now

		uptime := now.Sub(startTime)
		log.Printf("⚡ [运行 %v] 总请求=%d 成功=%d 失败=%d | RPS=%.1f/s | 平均=%v 最小=%v 最大=%v",
			uptime.Round(time.Second), total, success, fail,
			rps, avgDur.Round(time.Millisecond), minDur.Round(time.Millisecond), maxDur.Round(time.Millisecond))
		for _, fr := range failBuf {
			if fr.err != nil {
				log.Printf("  ✗ %s | 错误: %v | 耗时: %v", fr.name, fr.err, fr.elapsed.Round(time.Millisecond))
			} else {
				log.Printf("  ⚠ %s | HTTP %d | 耗时: %v | 响应: %s", fr.name, fr.status, fr.elapsed.Round(time.Millisecond), fr.body)
			}
		}
	}
}

func runRate(ratePerMinute, concurrency int) {
	intervalPerReq := time.Minute / time.Duration(ratePerMinute)
	log.Printf("📊 速率模式 | 每接口 %d 次/分钟 (每 %.0fms 发一次), 并发 %d",
		ratePerMinute, float64(intervalPerReq)/float64(time.Millisecond), concurrency)

	type task struct {
		name   string
		url    string
		method string
		body   string
	}

	tasks := []task{
		{"首页统计", statsAPIURL, "GET", ""},
		{"需求列表", demandAPIURL, "POST", demandPayload},
	}

	type stats struct {
		mu      sync.Mutex
		total   int
		success int
		fail    int
		failBuf []apiResult
	}

	s := &stats{}

	for _, t := range tasks {
		go func(tk task) {
			limiter := time.NewTicker(intervalPerReq)
			defer limiter.Stop()
			for range limiter.C {
				var wg sync.WaitGroup
				wg.Add(concurrency)
				for i := 0; i < concurrency; i++ {
					go func(idx int) {
						defer wg.Done()
						r := doRequest(tk.name, tk.url, tk.method, tk.body)
						s.mu.Lock()
						s.total++
						if r.err != nil || !r.success {
							s.fail++
							if len(s.failBuf) >= 5 {
								s.failBuf = s.failBuf[1:]
							}
							r.name = fmt.Sprintf("%s#%d", tk.name, idx+1)
							s.failBuf = append(s.failBuf, r)
						} else {
							s.success++
						}
						s.mu.Unlock()
					}(i)
				}
				wg.Wait()
			}
		}(t)
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	startTime := nowCST()
	lastTotal := 0
	lastTime := startTime

	for range ticker.C {
		s.mu.Lock()
		total := s.total
		success := s.success
		fail := s.fail
		failBuf := make([]apiResult, len(s.failBuf))
		copy(failBuf, s.failBuf)
		s.mu.Unlock()

		now := nowCST()
		elapsed := now.Sub(lastTime)
		intervalCount := total - lastTotal
		rps := float64(intervalCount) / elapsed.Seconds()
		lastTotal = total
		lastTime = now

		uptime := now.Sub(startTime)
		log.Printf("📊 [运行 %v] 总请求=%d 成功=%d 失败=%d | 实际RPS=%.2f/s (目标=%.2f/s)",
			uptime.Round(time.Second), total, success, fail,
			rps, float64(ratePerMinute*len(tasks))/60.0)
		for _, fr := range failBuf {
			if fr.err != nil {
				log.Printf("  ✗ %s | 错误: %v | 耗时: %v", fr.name, fr.err, fr.elapsed.Round(time.Millisecond))
			} else {
				log.Printf("  ⚠ %s | HTTP %d | 耗时: %v | 响应: %s", fr.name, fr.status, fr.elapsed.Round(time.Millisecond), fr.body)
			}
		}
	}
}

func main() {
	initChinaLoc()
	time.Local = chinaLoc

	log.Printf("[nettap] 启动 (version=%s)", version)

	cfg, err := loadConfig(defaultConfigPath)
	if err != nil {
		log.Printf("⚠ 配置文件加载失败，使用默认值: %v", err)
	}

	cfg.Timeout = func() int {
		if cfg.Timeout < 1 {
			return 15
		}
		return cfg.Timeout
	}()
	httpTimeout = time.Duration(cfg.Timeout) * time.Second

	cpus := runtime.NumCPU()
	if cfg.Concurrency < 1 {
		if cfg.FullPower {
			cfg.Concurrency = cpus * 2
		} else {
			cfg.Concurrency = 1
		}
	}

	log.Printf("配置文件: %s | CPU 核心: %d", defaultConfigPath, cpus)

	if cfg.FullPower {
		log.Printf("⚡ 全力模式 | 每接口并发 %d (自动=%d)", cfg.Concurrency, cpus*2)
		runFullPower(cfg.Concurrency)
		return
	}

	if cfg.RatePerMinute > 0 {
		if cfg.RatePerMinute > 60000 {
			log.Fatalf("rate_per_minute 不能超过 60000（每秒 1000 次）")
		}
		runRate(cfg.RatePerMinute, cfg.Concurrency)
		return
	}

	d, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		log.Fatalf("无效的 interval 值 %q: %v", cfg.Interval, err)
	}
	if d < time.Second {
		log.Fatalf("interval 不能小于 1 秒")
	}

	log.Printf("执行模式: 每 %v 一轮, 每接口并发 %d", d, cfg.Concurrency)

	if cfg.Once {
		runRound(cfg.Concurrency)
		return
	}

	runRound(cfg.Concurrency)

	next := nowCST().Add(d).Truncate(d)
	if !next.After(nowCST()) {
		next = nowCST().Add(d)
	}
	wait := time.Until(next)
	log.Printf("下次执行: %s (等待 %v)", next.Format("2006-01-02 15:04:05"), wait.Round(time.Second))
	time.Sleep(wait)

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	runRound(cfg.Concurrency)
	for range ticker.C {
		runRound(cfg.Concurrency)
		log.Printf("定时触发: %s", nowCST().Format("2006-01-02 15:04:05"))
	}
}
