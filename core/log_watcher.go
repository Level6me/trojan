package core

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	// Trojan C++: [2026-08-20 20:56:49] [INFO] 183.93.173.78:13876 authenticated by authenticator (3f1af17)
	// Trojan C++: [2026-08-20 20:56:49] [INFO] 183.93.173.78:13876 authenticated as (3f1af17)
	// Trojan C++: [2026-08-20 20:56:49] [INFO] [2001:db8::1]:13876 authenticated by authenticator (3f1af17)
	reTrojanCpp = regexp.MustCompile(`(?i)(?:\[([0-9a-fA-F:]+)\]|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})):\d+\s+authenticated(?:\s+by\s+authenticator|\s+as)?\s*\(?([a-fA-F0-9]{6,})\)?`)

	// Trojan-Go: [INFO] 2026/08/20 ... [user: user01] 183.93.173.78:12345 connected
	reTrojanGo1 = regexp.MustCompile(`(?i)\[user:\s*([^\s\]]+)\]\s*(?:\[([0-9a-fA-F:]+)\]|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})):\d+`)

	// Trojan-Go: 183.93.173.78:12345 authenticated as user user01
	reTrojanGo2 = regexp.MustCompile(`(?i)(?:\[([0-9a-fA-F:]+)\]|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})):\d+\s+authenticated\s+as\s+user\s+([^\s]+)`)

	// 日志时间提取: [2026-08-20 20:56:49] or 2026/08/20 20:56:49
	reLogTime = regexp.MustCompile(`(\d{4}[-/]\d{2}[-/]\d{2}\s+\d{2}:\d{2}:\d{2})`)
)

// ParseLogLine 解析单行 Trojan 日志并更新对应用户的活跃 IP
func ParseLogLine(line string) {
	if line == "" || (!strings.Contains(line, "authenticated") && !strings.Contains(line, "user:")) {
		return
	}

	mysql := GetMysql()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if tmMatch := reLogTime.FindStringSubmatch(line); len(tmMatch) > 1 {
		tStr := strings.ReplaceAll(tmMatch[1], "/", "-")
		if _, err := time.Parse("2006-01-02 15:04:05", tStr); err == nil {
			nowStr = tStr
		}
	}

	// 1. Trojan C++
	if m := reTrojanCpp.FindStringSubmatch(line); len(m) >= 4 {
		ip := m[1]
		if ip == "" {
			ip = m[2]
		}
		hashPrefix := m[3]
		if ip != "" && hashPrefix != "" {
			_ = mysql.UpdateUserLastIPByHashPrefix(hashPrefix, ip, nowStr)
			return
		}
	}

	// 2. Trojan-Go pattern 1
	if m := reTrojanGo1.FindStringSubmatch(line); len(m) >= 4 {
		username := m[1]
		ip := m[2]
		if ip == "" {
			ip = m[3]
		}
		if username != "" && ip != "" {
			_ = mysql.UpdateUserLastIPByUsername(username, ip, nowStr)
			return
		}
	}

	// 3. Trojan-Go pattern 2
	if m := reTrojanGo2.FindStringSubmatch(line); len(m) >= 4 {
		ip := m[1]
		if ip == "" {
			ip = m[2]
		}
		username := m[3]
		if username != "" && ip != "" {
			_ = mysql.UpdateUserLastIPByUsername(username, ip, nowStr)
			return
		}
	}
}

// catchupHistoricalLogs 读取近期 N 条日志进行历史追溯
func catchupHistoricalLogs(serviceName string, n int) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("journalctl -u %s -n %d -o cat", serviceName, n))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		ParseLogLine(scanner.Text())
	}
	_ = cmd.Wait()
}

// followServiceLog 持续监听指定服务的实时日志流
func followServiceLog(serviceName string) {
	for {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("journalctl -f -u %s -o cat -n 0", serviceName))
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ParseLogLine(scanner.Text())
		}
		_ = cmd.Wait()
		time.Sleep(3 * time.Second)
	}
}

// StartLogWatcher 启动 Trojan / Trojan-Go 日志监听与 IP 自动记录
func StartLogWatcher() {
	services := []string{"trojan", "trojan-go"}

	// 1. 启动时先回溯历史日志，确保已有活跃用户立刻显示 IP
	go func() {
		time.Sleep(1 * time.Second)
		for _, svc := range services {
			catchupHistoricalLogs(svc, 300)
		}
	}()

	// 2. 为每个服务分别启动持久监听通道
	for _, svc := range services {
		go followServiceLog(svc)
	}
}
