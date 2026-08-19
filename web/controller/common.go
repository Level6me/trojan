package controller

import (
	"crypto/x509"
	"encoding/pem"
	"os"
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
	data := map[string]interface{}{
		"certPath":      certPath,
		"keyPath":       config.SSl.Key,
		"exists":        false,
		"domain":        "",
		"issuer":        "",
		"notBefore":     "",
		"notAfter":      "",
		"daysRemaining": 0,
		"isExpired":     false,
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
				}
			}
		}
	}
	responseBody.Data = data
	return &responseBody
}
