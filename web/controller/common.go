package controller

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"trojan/asset"
	"trojan/core"
	"trojan/trojan"
	"trojan/util"
)

// ResponseBody 结构体
type ResponseBody struct {
	Duration string
	Data     interface{}
	Msg      string
}

type speedInfo struct {
	Up   uint64
	Down uint64
}

var si *speedInfo

// TimeCost web函数执行用时统计方法
func TimeCost(start time.Time, body *ResponseBody) {
	body.Duration = time.Since(start).String()
}

func clashRules() string {
	rules, _ := core.GetValue("clash-rules")
	if rules == "" {
		rules = string(asset.GetAsset("clash-rules.yaml"))
	}
	return rules
}

// Version 获取版本信息
func Version() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = map[string]string{
		"version":       trojan.MVersion,
		"buildDate":     trojan.BuildDate,
		"goVersion":     trojan.GoVersion,
		"gitVersion":    trojan.GitVersion,
		"trojanVersion": trojan.Version(),
		"trojanUptime":  trojan.UpTime(),
		"trojanType":    trojan.Type(),
	}
	return &responseBody
}

// SetLoginInfo 设置登录页信息
func SetLoginInfo(title string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := core.SetValue("login_title", title)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// SetDomain 设置域名
func SetDomain(domain string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.SetDomain(domain)
	return &responseBody
}

// SetClashRules 设置clash规则
func SetClashRules(rules string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	core.SetValue("clash-rules", rules)
	return &responseBody
}

// ResetClashRules 重置clash规则
func ResetClashRules() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	core.DelValue("clash-rules")
	responseBody.Data = clashRules()
	return &responseBody
}

// GetClashRules 获取clash规则
func GetClashRules() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = clashRules()
	return &responseBody
}

// SetTrojanType 设置trojan类型
func SetTrojanType(tType string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := trojan.SwitchType(tType)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CollectTask 启动收集主机信息任务
func CollectTask() {
	var recvCount, sentCount uint64
	c := cron.New()
	lastIO, _ := net.IOCounters(true)
	var lastRecvCount, lastSentCount uint64
	for _, k := range lastIO {
		lastRecvCount = lastRecvCount + k.BytesRecv
		lastSentCount = lastSentCount + k.BytesSent
	}
	si = &speedInfo{}
	c.AddFunc("@every 2s", func() {
		result, _ := net.IOCounters(true)
		recvCount, sentCount = 0, 0
		for _, k := range result {
			recvCount = recvCount + k.BytesRecv
			sentCount = sentCount + k.BytesSent
		}
		si.Up = (sentCount - lastSentCount) / 2
		si.Down = (recvCount - lastRecvCount) / 2
		lastSentCount = sentCount
		lastRecvCount = recvCount
		lastIO = result
	})
	c.Start()
}

// ServerInfo 获取服务器信息
func ServerInfo() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	cpuPercent, _ := cpu.Percent(0, false)
	vmInfo, _ := mem.VirtualMemory()
	smInfo, _ := mem.SwapMemory()
	diskInfo, _ := disk.Usage("/")
	loadInfo, _ := load.Avg()
	tcpCon, _ := net.Connections("tcp")
	udpCon, _ := net.Connections("udp")
	netCount := map[string]int{
		"tcp": len(tcpCon),
		"udp": len(udpCon),
	}
	responseBody.Data = map[string]interface{}{
		"cpu":      cpuPercent,
		"memory":   vmInfo,
		"swap":     smInfo,
		"disk":     diskInfo,
		"load":     loadInfo,
		"speed":    si,
		"netCount": netCount,
	}
	return &responseBody
}

// CertInfo 获取TLS证书详细信息与到期天数
func CertInfo() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	config := core.GetConfig()
	certPath := config.SSl.Cert
	
	autoRenewStr, _ := core.GetValue("auto_renew_cert")
	autoRenew := autoRenewStr != "false" // 默认开启

	data := map[string]interface{}{
		"certPath":      certPath,
		"keyPath":       config.SSl.Key,
		"sni":           config.SSl.Sni,
		"exists":        false,
		"domain":        "",
		"issuer":        "",
		"notBefore":     "",
		"notAfter":      "",
		"daysRemaining": 0,
		"isExpired":     false,
		"autoRenew":     autoRenew,
		"keyType":       "ECC 256 / RSA",
		"sigAlg":        "",
	}

	if certPath != "" {
		if certBytes, err := os.ReadFile(certPath); err == nil {
			data["exists"] = true
			block, _ := pem.Decode(certBytes)
			if block != nil {
				if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
					data["domain"] = cert.Subject.CommonName
					if len(cert.DNSNames) > 0 {
						data["dnsNames"] = cert.DNSNames
					}
					data["issuer"] = cert.Issuer.CommonName
					data["notBefore"] = cert.NotBefore.Format("2006-01-02 15:04:05")
					data["notAfter"] = cert.NotAfter.Format("2006-01-02 15:04:05")
					days := int(time.Until(cert.NotAfter).Hours() / 24)
					data["daysRemaining"] = days
					data["isExpired"] = time.Now().After(cert.NotAfter)
					data["sigAlg"] = cert.SignatureAlgorithm.String()

					if pubKey := cert.PublicKey; pubKey != nil {
						switch k := pubKey.(type) {
						case *rsa.PublicKey:
							data["keyType"] = fmt.Sprintf("RSA %d bits", k.N.BitLen())
						case *ecdsa.PublicKey:
							data["keyType"] = fmt.Sprintf("ECDSA %s", k.Curve.Params().Name)
						default:
							data["keyType"] = "ECC Standard"
						}
					}
				}
			}
		}
	}
	responseBody.Data = data
	return &responseBody
}

// SetAutoRenewCert 开启或关闭证书自动续签
func SetAutoRenewCert(enabled bool) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	val := "false"
	if enabled {
		val = "true"
	}
	if err := core.SetValue("auto_renew_cert", val); err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	// 同步更新系统 crontab 定时任务
	syncAcmeCrontab(enabled)

	return &responseBody
}

// syncAcmeCrontab 同步 acme.sh 续签任务到系统 crontab
func syncAcmeCrontab(enabled bool) {
	currentCron := util.ExecCommandWithResult("crontab -l 2>/dev/null || true")
	lines := strings.Split(currentCron, "\n")
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 过滤掉已存在的 acme.sh 任务行
		if strings.Contains(trimmed, "acme.sh") {
			continue
		}
		newLines = append(newLines, trimmed)
	}

	if enabled {
		acmePath := "/root/.acme.sh/acme.sh"
		if !util.IsExists(acmePath) {
			acmePath = "acme.sh"
		}
		cronLine := fmt.Sprintf("0 15 * * * systemctl stop trojan-web 2>/dev/null; \"%s\" --cron --home \"/root/.acme.sh\" >/dev/null 2>&1; systemctl start trojan-web 2>/dev/null", acmePath)
		newLines = append(newLines, cronLine)
	}

	finalCron := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		finalCron += "\n"
	}
	_ = util.ExecCommand(fmt.Sprintf("echo '%s' | crontab -", strings.ReplaceAll(finalCron, "'", "'\\''")))
}

// RenewCert 手动立即续签证书
func RenewCert() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	config := core.GetConfig()
	domain := config.SSl.Sni
	if domain == "" {
		if config.SSl.Cert != "" {
			if certBytes, err := os.ReadFile(config.SSl.Cert); err == nil {
				if block, _ := pem.Decode(certBytes); block != nil {
					if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
						domain = cert.Subject.CommonName
					}
				}
			}
		}
	}

	acmePath := "/root/.acme.sh/acme.sh"
	if !util.IsExists(acmePath) {
		acmePath = "acme.sh"
	}

	var cmd string
	if domain != "" {
		cmd = fmt.Sprintf("systemctl stop trojan-web 2>/dev/null; \"%s\" --renew -d %s --force --home \"/root/.acme.sh\"; systemctl start trojan-web 2>/dev/null", acmePath, domain)
	} else {
		cmd = fmt.Sprintf("systemctl stop trojan-web 2>/dev/null; \"%s\" --cron --force --home \"/root/.acme.sh\"; systemctl start trojan-web 2>/dev/null", acmePath)
	}

	out := util.ExecCommandWithResult(cmd)
	// 重启 Trojan 内核重载证书
	trojan.Restart()

	responseBody.Data = map[string]interface{}{
		"output": out,
	}
	return &responseBody
}
