package controller

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
	"io"
	"strconv"
	"strings"
	"time"
	"trojan/core"
	"trojan/trojan"
	"trojan/util"
)

// Start 启动trojan
func Start() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.Start()
	return &responseBody
}

// Stop 停止trojan
func Stop() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.Stop()
	return &responseBody
}

// Restart 重启trojan
func Restart() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.Restart()
	return &responseBody
}

// Update trojan更新
func Update() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.InstallTrojan("")
	return &responseBody
}

// SetLogLevel 修改trojan日志等级
func SetLogLevel(level int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	core.WriteLogLevel(level)
	trojan.Restart()
	return &responseBody
}

// GetLogLevel 获取trojan日志等级
func GetLogLevel() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	config := core.GetConfig()
	responseBody.Data = map[string]interface{}{
		"loglevel": &config.LogLevel,
	}
	return &responseBody
}

// Log 通过ws查看trojan实时日志
func Log(c *gin.Context) {
	var (
		wsConn *util.WsConnection
		err    error
	)
	if wsConn, err = util.InitWebsocket(c.Writer, c.Request); err != nil {
		fmt.Println(err)
		return
	}
	defer wsConn.WsClose()
	param := c.DefaultQuery("line", "300")
	if !util.IsInteger(param) {
		fmt.Println("invalid param: " + param)
		return
	}
	if param == "-1" {
		param = "--no-tail"
	} else {
		param = "-n " + param
	}
	result, err := util.LogChan("trojan", param, wsConn.CloseChan)
	if err != nil {
		fmt.Println(err)
		return
	}
	for line := range result {
		if err := wsConn.WsWrite(ws.TextMessage, []byte(line+"\n")); err != nil {
			fmt.Println("can't send: ", line)
			break
		}
	}
}

// ImportCsv 导入csv文件到trojan数据库 (支持智能表头识别、追加/更新模式)
func ImportCsv(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		responseBody.Msg = "无法读取上传文件: " + err.Error()
		return &responseBody
	}
	defer file.Close()
	filename := header.Filename
	if !strings.Contains(strings.ToLower(filename), ".csv") {
		responseBody.Msg = "仅支持导入 CSV 格式的文件 (.csv)"
		return &responseBody
	}

	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1 // 允许变长记录
	reader.LazyQuotes = true

	mysql := core.GetMysql()
	mysql.CreateTable() // 确保数据表与字段存在

	var records [][]string
	for {
		line, readErr := reader.Read()
		if readErr == io.EOF {
			break
		} else if readErr != nil {
			responseBody.Msg = "解析 CSV 异常: " + readErr.Error()
			return &responseBody
		}
		if len(line) == 0 {
			continue
		}
		// 去除第一列的首字符 BOM (若存在)
		if len(records) == 0 && len(line) > 0 {
			line[0] = strings.TrimPrefix(line[0], "\xef\xbb\xbf")
		}
		records = append(records, line)
	}

	if len(records) == 0 {
		responseBody.Msg = "CSV 文件内容为空"
		return &responseBody
	}

	// 智能字段列索引映射
	colUser := -1
	colPass := -1
	colQuota := -1
	colDays := -1
	colExpiry := -1
	colSpeed := -1

	startRow := 0
	firstRow := records[0]

	// 检查第一行是否为表头
	hasHeader := false
	for idx, val := range firstRow {
		v := strings.ToLower(strings.TrimSpace(val))
		if strings.Contains(v, "user") || strings.Contains(v, "用户名") || strings.Contains(v, "账户") {
			colUser = idx
			hasHeader = true
		} else if strings.Contains(v, "pass") || strings.Contains(v, "密码") {
			colPass = idx
			hasHeader = true
		} else if strings.Contains(v, "quota") || strings.Contains(v, "配额") || strings.Contains(v, "流量") {
			colQuota = idx
		} else if strings.Contains(v, "day") || strings.Contains(v, "天数") {
			colDays = idx
		} else if strings.Contains(v, "expire") || strings.Contains(v, "到期") || strings.Contains(v, "有效") {
			colExpiry = idx
		} else if strings.Contains(v, "speed") || strings.Contains(v, "限速") {
			colSpeed = idx
		}
	}

	if hasHeader {
		startRow = 1
	} else {
		// 默认列回退（无表头时按标准顺序：用户名, 密码, 配额, 天数, 到期时间, 限速）
		colUser = 0
		if len(firstRow) > 1 {
			colPass = 1
		}
		if len(firstRow) > 2 {
			colQuota = 2
		}
		if len(firstRow) > 3 {
			colDays = 3
		}
		if len(firstRow) > 4 {
			colExpiry = 4
		}
		if len(firstRow) > 5 {
			colSpeed = 5
		}
	}

	if colUser == -1 {
		colUser = 0
	}
	if colPass == -1 {
		colPass = 1
	}

	importedCount := 0
	updatedCount := 0

	for i := startRow; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}
		username := ""
		if colUser < len(row) {
			username = strings.TrimSpace(row[colUser])
		}
		if username == "" || username == "admin" {
			continue
		}

		plainPass := ""
		if colPass < len(row) {
			plainPass = strings.TrimSpace(row[colPass])
		}
		if plainPass == "" {
			plainPass = util.RandString(8, util.LETTER+util.DIGITS)
		}

		var quota int64 = 0
		if colQuota != -1 && colQuota < len(row) {
			qStr := strings.TrimSpace(row[colQuota])
			if qVal, err := strconv.ParseInt(qStr, 10, 64); err == nil {
				// 如果小于 1000 则可能单位为 GB
				if qVal > 0 && qVal < 10000 {
					quota = qVal * 1024 * 1024 * 1024
				} else {
					quota = qVal
				}
			}
		}

		var useDays uint = 0
		if colDays != -1 && colDays < len(row) {
			dStr := strings.TrimSpace(row[colDays])
			if dVal, err := strconv.Atoi(dStr); err == nil && dVal >= 0 {
				useDays = uint(dVal)
			}
		}

		expiryDate := ""
		if colExpiry != -1 && colExpiry < len(row) {
			expiryDate = strings.TrimSpace(row[colExpiry])
		}
		if expiryDate == "" && useDays > 0 {
			now := time.Now()
			utc, _ := time.LoadLocation("Asia/Shanghai")
			addDay, _ := time.ParseDuration(strconv.Itoa(int(24*useDays)) + "h")
			expiryDate = now.Add(addDay).In(utc).Format("2006-01-02")
		}

		var speedLimit int = 0
		if colSpeed != -1 && colSpeed < len(row) {
			sStr := strings.TrimSpace(row[colSpeed])
			if sVal, err := strconv.Atoi(sStr); err == nil {
				speedLimit = sVal
			}
		}

		base64Pass := base64.StdEncoding.EncodeToString([]byte(plainPass))
		encryPass := fmt.Sprintf("%x", sha256.Sum224([]byte(plainPass)))

		existing := mysql.GetUserByName(username)
		if existing != nil {
			// 更新已有用户
			db := mysql.GetDB()
			if db != nil {
				_, _ = db.Exec(fmt.Sprintf(`
					UPDATE users SET
					password = '%s',
					passwordShow = '%s',
					quota = %d,
					useDays = %d,
					expiryDate = '%s',
					speedLimit = %d
					WHERE id = %d;
				`, encryPass, base64Pass, quota, useDays, expiryDate, speedLimit, existing.ID))
				db.Close()
				updatedCount++
			}
		} else {
			// 插入新用户
			db := mysql.GetDB()
			if db != nil {
				_, _ = db.Exec(fmt.Sprintf(`
					INSERT INTO users(username, password, passwordShow, quota, useDays, expiryDate, speedLimit)
					VALUES ('%s', '%s', '%s', %d, %d, '%s', %d);
				`, username, encryPass, base64Pass, quota, useDays, expiryDate, speedLimit))
				db.Close()
				importedCount++
			}
		}
	}

	// 重启 Trojan 服务加载新用户凭证
	trojan.Restart()

	responseBody.Data = map[string]interface{}{
		"total":    importedCount + updatedCount,
		"imported": importedCount,
		"updated":  updatedCount,
	}
	return &responseBody
}

// ExportCsv 导出trojan表数据到csv文件
func ExportCsv(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	var dataBytes = new(bytes.Buffer)
	// 设置 UTF-8 BOM, 防止 Excel 打开中文乱码
	dataBytes.WriteString("\xEF\xBB\xBF")
	mysql := core.GetMysql()
	userList, err := mysql.GetData()
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	wr := csv.NewWriter(dataBytes)
	// 写入表头
	wr.Write([]string{
		"ID",
		"用户名",
		"节点密码",
		"流量配额(字节)",
		"已用下载(字节)",
		"已用上传(字节)",
		"可用天数",
		"到期时间",
		"限速(KB/s)",
		"历史使用时间",
		"历史使用IP",
		"最高下载速率(B/s)",
		"最高上传速率(B/s)",
	})
	for _, user := range userList {
		pass, _ := base64.StdEncoding.DecodeString(user.Password)
		plainPass := string(pass)
		if plainPass == "" {
			plainPass = user.Password
		}
		singleUser := []string{
			strconv.Itoa(int(user.ID)),
			user.Username,
			plainPass,
			strconv.FormatInt(user.Quota, 10),
			strconv.FormatUint(user.Download, 10),
			strconv.FormatUint(user.Upload, 10),
			strconv.Itoa(int(user.UseDays)),
			user.ExpiryDate,
			strconv.Itoa(user.SpeedLimit),
			user.LastActiveTime,
			user.LastIP,
			strconv.FormatUint(user.PeakDownSpeed, 10),
			strconv.FormatUint(user.PeakUpSpeed, 10),
		}
		wr.Write(singleUser)
	}
	wr.Flush()
	c.Writer.Header().Set("Content-type", "application/octet-stream")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", fmt.Sprintf("%s_users_%s.csv", mysql.Database, time.Now().Format("20060102"))))
	c.String(200, dataBytes.String())
	return nil
}
