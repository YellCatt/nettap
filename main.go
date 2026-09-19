package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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

type apiResult struct {
	name     string
	url      string
	status   int
	elapsed  time.Duration
	err      error
	bodySize int
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
	req.Header.Set("User-Agent", "Nettap/1.0")

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
			log.Printf("  ⚠ %s | HTTP %d | 耗时: %v | 响应: %d 字节", r.name, r.status, r.elapsed.Round(time.Millisecond), r.bodySize)
		} else {
			success++
			log.Printf("  ✓ %s | HTTP %d | 耗时: %v | 响应: %d 字节", r.name, r.status, r.elapsed.Round(time.Millisecond), r.bodySize)
		}
	}
	log.Printf("========== 本轮完成: 共 %d 请求, 成功 %d, 失败 %d ==========", total, success, fail)
}

func main() {
	once := flag.Bool("once", false, "只执行一轮后退出")
	interval := flag.String("interval", "", "执行间隔，如 30s / 5m / 1h（覆盖 config.yaml）")
	concurrency := flag.Int("c", 0, "每个接口的并发请求数（覆盖 config.yaml）")
	configPath := flag.String("config", defaultConfigPath, "配置文件路径")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()

	initChinaLoc()
	time.Local = chinaLoc

	if *showVersion {
		log.Printf("[nettap] version=%s", version)
		return
	}

	log.Printf("[nettap] 启动 (version=%s)", version)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Printf("⚠ 配置文件加载失败，使用默认值: %v", err)
	}

	if *interval != "" {
		cfg.Interval = *interval
	}
	if *concurrency > 0 {
		cfg.Concurrency = *concurrency
	}
	if *once {
		cfg.Once = true
	}

	d, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		log.Fatalf("无效的 interval 值 %q: %v（支持的格式: 30s, 5m, 1h, 2h30m 等）", cfg.Interval, err)
	}
	if d < time.Second {
		log.Fatalf("interval 不能小于 1 秒")
	}
	if cfg.Concurrency < 1 {
		log.Fatalf("concurrency 不能小于 1")
	}

	cfg.Timeout = func() int {
		if cfg.Timeout < 1 {
			return 15
		}
		return cfg.Timeout
	}()
	httpTimeout = time.Duration(cfg.Timeout) * time.Second

	log.Printf("配置文件: %s", *configPath)
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
