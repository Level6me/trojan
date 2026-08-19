package core

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	// mysql sql驱动
	_ "github.com/go-sql-driver/mysql"
)

// Mysql 结构体
type Mysql struct {
	Enabled    bool   `json:"enabled"`
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	Database   string `json:"database"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Cafile     string `json:"cafile"`
}

// User 用户表记录结构体
type User struct {
	ID          uint
	Username    string
	Password    string
	EncryptPass string
	Quota       int64
	Download    uint64
	Upload      uint64
	UseDays     uint
	ExpiryDate  string
}

// PageQuery 分页查询的结构体
type PageQuery struct {
	PageNum  int
	CurPage  int
	Total    int
	PageSize int
	DataList []*User
}

// DailyTraffic 每日流量明细结构体
type DailyTraffic struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	LogDate  string `json:"log_date"`
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
	Total    uint64 `json:"total"`
}

// GlobalDailyTraffic 全站每日流量汇总
type GlobalDailyTraffic struct {
	LogDate  string `json:"log_date"`
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
	Total    uint64 `json:"total"`
}

// GlobalHourlyTraffic 全站小时级流量汇总
type GlobalHourlyTraffic struct {
	LogTime  string `json:"log_time"` // 格式 "15:00"
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
	Total    uint64 `json:"total"`
}

// CreateTableSql 创表sql
var CreateTableSql = `
CREATE TABLE IF NOT EXISTS users (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password CHAR(56) NOT NULL,
    passwordShow VARCHAR(255) NOT NULL,
    quota BIGINT NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    useDays int(10) DEFAULT 0,
    expiryDate char(10) DEFAULT '',
    PRIMARY KEY (id),
    INDEX (password)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_traffic_daily (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id INT UNSIGNED NOT NULL,
    username VARCHAR(64) NOT NULL,
    log_date VARCHAR(10) NOT NULL,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    total BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY idx_user_date (user_id, log_date),
    INDEX idx_log_date (log_date)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_traffic_hourly (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id INT UNSIGNED NOT NULL,
    username VARCHAR(64) NOT NULL,
    log_time VARCHAR(16) NOT NULL,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    total BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY idx_user_hour (user_id, log_time),
    INDEX idx_log_hour (log_time)
) DEFAULT CHARSET=utf8mb4;
`

// GetDB 获取mysql数据库连接
func (mysql *Mysql) GetDB() *sql.DB {
	// 屏蔽mysql驱动包的日志输出
	mysqlDriver.SetLogger(log.New(io.Discard, "", 0))
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", mysql.Username, mysql.Password, mysql.ServerAddr, mysql.ServerPort, mysql.Database)
	db, err := sql.Open("mysql", conn)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return db
}

// CreateTable 不存在trojan user表及流量表则自动创建
func (mysql *Mysql) CreateTable() {
	db := mysql.GetDB()
	if db == nil {
		return
	}
	defer db.Close()
	queries := strings.Split(CreateTableSql, ";")
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q != "" {
			if _, err := db.Exec(q); err != nil {
				fmt.Println("CreateTable Error:", err)
			}
		}
	}
}

func queryUserList(db *sql.DB, sql string) ([]*User, error) {
	var (
		username    string
		encryptPass string
		passShow    string
		download    uint64
		upload      uint64
		quota       int64
		id          uint
		useDays     uint
		expiryDate  string
	)
	var userList []*User
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&id, &username, &encryptPass, &passShow, &quota, &download, &upload, &useDays, &expiryDate); err != nil {
			return nil, err
		}
		userList = append(userList, &User{
			ID:          id,
			Username:    username,
			Password:    passShow,
			EncryptPass: encryptPass,
			Download:    download,
			Upload:      upload,
			Quota:       quota,
			UseDays:     useDays,
			ExpiryDate:  expiryDate,
		})
	}
	return userList, nil
}

func queryUser(db *sql.DB, sql string) (*User, error) {
	var (
		username    string
		encryptPass string
		passShow    string
		download    uint64
		upload      uint64
		quota       int64
		id          uint
		useDays     uint
		expiryDate  string
	)
	row := db.QueryRow(sql)
	if err := row.Scan(&id, &username, &encryptPass, &passShow, &quota, &download, &upload, &useDays, &expiryDate); err != nil {
		return nil, err
	}
	return &User{ID: id, Username: username, Password: passShow, EncryptPass: encryptPass, Download: download, Upload: upload, Quota: quota, UseDays: useDays, ExpiryDate: expiryDate}, nil
}

// CreateUser 创建Trojan用户
func (mysql *Mysql) CreateUser(username string, base64Pass string, originPass string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	encryPass := sha256.Sum224([]byte(originPass))
	if _, err := db.Exec(fmt.Sprintf("INSERT INTO users(username, password, passwordShow, quota) VALUES ('%s', '%x', '%s', -1);", username, encryPass, base64Pass)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// UpdateUser 更新Trojan用户名和密码
func (mysql *Mysql) UpdateUser(id uint, username string, base64Pass string, originPass string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	encryPass := sha256.Sum224([]byte(originPass))
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET username='%s', password='%x', passwordShow='%s' WHERE id=%d;", username, encryPass, base64Pass, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// DeleteUser 删除用户
func (mysql *Mysql) DeleteUser(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if userList, err := mysql.GetData(strconv.Itoa(int(id))); err != nil {
		return err
	} else if userList != nil && len(userList) == 0 {
		return fmt.Errorf("不存在id为%d的用户", id)
	}
	if _, err := db.Exec(fmt.Sprintf("DELETE FROM users WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// MonthlyResetData 设置了过期时间的用户，每月定时清空使用流量
func (mysql *Mysql) MonthlyResetData() error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	userList, err := queryUserList(db, "SELECT * FROM users WHERE useDays != 0 AND quota != 0")
	if err != nil {
		return err
	}
	for _, user := range userList {
		if _, err := db.Exec(fmt.Sprintf("UPDATE users SET download=0, upload=0 WHERE id=%d;", user.ID)); err != nil {
			return err
		}
	}
	return nil
}

// DailyCheckExpire 检查是否有过期，过期了设置流量上限为0
func (mysql *Mysql) DailyCheckExpire() (bool, error) {
	needRestart := false
	now := time.Now()
	utc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return false, err
	}
	addDay, _ := time.ParseDuration("-24h")
	yesterdayStr := now.Add(addDay).In(utc).Format("2006-01-02")
	yesterday, _ := time.Parse("2006-01-02", yesterdayStr)
	db := mysql.GetDB()
	if db == nil {
		return false, errors.New("can't connect mysql")
	}
	defer db.Close()
	userList, err := queryUserList(db, "SELECT * FROM users WHERE quota != 0")
	if err != nil {
		return false, err
	}
	for _, user := range userList {
		if expireDate, err := time.Parse("2006-01-02", user.ExpiryDate); err == nil {
			if yesterday.Sub(expireDate).Seconds() >= 0 {
				if _, err := db.Exec(fmt.Sprintf("UPDATE users SET quota=0 WHERE id=%d;", user.ID)); err != nil {
					return false, err
				}
				if !needRestart {
					needRestart = true
				}
			}
		}
	}
	return needRestart, nil
}

// CancelExpire 取消过期时间
func (mysql *Mysql) CancelExpire(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET useDays=0, expiryDate='' WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// SetExpire 设置过期时间
func (mysql *Mysql) SetExpire(id uint, useDays uint) error {
	now := time.Now()
	utc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println(err)
		return err
	}
	addDay, _ := time.ParseDuration(strconv.Itoa(int(24*useDays)) + "h")
	expiryDate := now.Add(addDay).In(utc).Format("2006-01-02")

	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET useDays=%d, expiryDate='%s' WHERE id=%d;", useDays, expiryDate, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// SetQuota 限制流量
func (mysql *Mysql) SetQuota(id uint, quota int) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET quota=%d WHERE id=%d;", quota, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// CleanData 清空流量统计
func (mysql *Mysql) CleanData(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET download=0, upload=0 WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// CleanDataByName 清空指定用户名流量统计数据
func (mysql *Mysql) CleanDataByName(usernames []string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	runSql := "UPDATE users SET download=0, upload=0 WHERE BINARY username in ("
	for i, name := range usernames {
		runSql = runSql + "'" + name + "'"
		if i == len(usernames)-1 {
			runSql = runSql + ")"
		} else {
			runSql = runSql + ","
		}
	}
	if _, err := db.Exec(runSql); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// GetUserByName 通过用户名来获取用户
func (mysql *Mysql) GetUserByName(name string) *User {
	db := mysql.GetDB()
	if db == nil {
		return nil
	}
	defer db.Close()
	user, err := queryUser(db, fmt.Sprintf("SELECT * FROM users WHERE BINARY username='%s'", name))
	if err != nil {
		return nil
	}
	return user
}

// GetUserByPass 通过密码来获取用户
func (mysql *Mysql) GetUserByPass(pass string) *User {
	db := mysql.GetDB()
	if db == nil {
		return nil
	}
	defer db.Close()
	user, err := queryUser(db, fmt.Sprintf("SELECT * FROM users WHERE BINARY passwordShow='%s'", pass))
	if err != nil {
		return nil
	}
	return user
}

// PageList 通过分页获取用户记录
func (mysql *Mysql) PageList(curPage int, pageSize int) (*PageQuery, error) {
	var (
		total int
	)

	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("连接mysql失败")
	}
	defer db.Close()
	offset := (curPage - 1) * pageSize
	querySQL := fmt.Sprintf("SELECT * FROM users LIMIT %d, %d", offset, pageSize)
	userList, err := queryUserList(db, querySQL)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	db.QueryRow("SELECT COUNT(id) FROM users").Scan(&total)
	return &PageQuery{
		CurPage:  curPage,
		PageSize: pageSize,
		Total:    total,
		DataList: userList,
		PageNum:  (total + pageSize - 1) / pageSize,
	}, nil
}

// GetData 获取用户记录
func (mysql *Mysql) GetData(ids ...string) ([]*User, error) {
	querySQL := "SELECT * FROM users"
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("连接mysql失败")
	}
	defer db.Close()
	if len(ids) > 0 {
		querySQL = querySQL + " WHERE id in (" + strings.Join(ids, ",") + ")"
	}
	userList, err := queryUserList(db, querySQL)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return userList, nil
}

var (
	lastTrafficLock sync.Mutex
	lastUserTraffic = make(map[uint][2]uint64)
)

// RecordTrafficSnapshot 采样并累加计算每日与每小时流量快照
func (mysql *Mysql) RecordTrafficSnapshot() error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()

	users, err := queryUserList(db, "SELECT * FROM users")
	if err != nil {
		return err
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	thisHour := now.Format("2006-01-02 15:00")

	lastTrafficLock.Lock()
	defer lastTrafficLock.Unlock()

	for _, u := range users {
		last, exists := lastUserTraffic[u.ID]
		var deltaUp, deltaDown uint64

		if !exists {
			lastUserTraffic[u.ID] = [2]uint64{u.Upload, u.Download}
			deltaUp = 0
			deltaDown = 0
		} else {
			if u.Upload >= last[0] {
				deltaUp = u.Upload - last[0]
			} else {
				deltaUp = u.Upload
			}

			if u.Download >= last[1] {
				deltaDown = u.Download - last[1]
			} else {
				deltaDown = u.Download
			}
			lastUserTraffic[u.ID] = [2]uint64{u.Upload, u.Download}
		}

		// 写入每日流量表
		queryDaily := fmt.Sprintf(`
			INSERT INTO user_traffic_daily (user_id, username, log_date, upload, download, total)
			VALUES (%d, '%s', '%s', %d, %d, %d)
			ON DUPLICATE KEY UPDATE
			username = VALUES(username),
			upload = upload + %d,
			download = download + %d,
			total = total + %d;
		`, u.ID, u.Username, today, deltaUp, deltaDown, deltaUp+deltaDown, deltaUp, deltaDown, deltaUp+deltaDown)
		db.Exec(queryDaily)

		// 写入每小时流量表
		queryHourly := fmt.Sprintf(`
			INSERT INTO user_traffic_hourly (user_id, username, log_time, upload, download, total)
			VALUES (%d, '%s', '%s', %d, %d, %d)
			ON DUPLICATE KEY UPDATE
			username = VALUES(username),
			upload = upload + %d,
			download = download + %d,
			total = total + %d;
		`, u.ID, u.Username, thisHour, deltaUp, deltaDown, deltaUp+deltaDown, deltaUp, deltaDown, deltaUp+deltaDown)
		db.Exec(queryHourly)
	}
	return nil
}

// GetGlobalDailyTraffic 获取全站近 N 天每日总流量（自动补齐连续日期）
func (mysql *Mysql) GetGlobalDailyTraffic(days int) ([]*GlobalDailyTraffic, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()

	if days <= 0 {
		days = 7
	}

	startDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	query := fmt.Sprintf(`
		SELECT log_date, SUM(upload) as up, SUM(download) as down, SUM(total) as tot
		FROM user_traffic_daily
		WHERE log_date >= '%s'
		GROUP BY log_date
		ORDER BY log_date ASC
	`, startDate)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataMap := make(map[string]*GlobalDailyTraffic)
	for rows.Next() {
		var logDate string
		var up, down, tot uint64
		if err := rows.Scan(&logDate, &up, &down, &tot); err == nil {
			dataMap[logDate] = &GlobalDailyTraffic{
				LogDate:  logDate,
				Upload:   up,
				Download: down,
				Total:    tot,
			}
		}
	}

	// 补齐连续 days 天平滑数据
	now := time.Now()
	var list []*GlobalDailyTraffic
	for i := days - 1; i >= 0; i-- {
		dStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		if item, ok := dataMap[dStr]; ok {
			list = append(list, item)
		} else {
			list = append(list, &GlobalDailyTraffic{
				LogDate:  dStr,
				Upload:   0,
				Download: 0,
				Total:    0,
			})
		}
	}
	return list, nil
}

// GetGlobalHourlyTraffic 获取全站近 N 小时每小时总流量（自动补齐 24 小时整点连续序列）
func (mysql *Mysql) GetGlobalHourlyTraffic(hours int) ([]*GlobalHourlyTraffic, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()

	if hours <= 0 {
		hours = 24
	}

	now := time.Now()
	startHour := now.Add(-time.Duration(hours-1) * time.Hour).Format("2006-01-02 15:00")
	query := fmt.Sprintf(`
		SELECT log_time, SUM(upload) as up, SUM(download) as down, SUM(total) as tot
		FROM user_traffic_hourly
		WHERE log_time >= '%s'
		GROUP BY log_time
		ORDER BY log_time ASC
	`, startHour)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataMap := make(map[string]*GlobalHourlyTraffic)
	for rows.Next() {
		var logTime string
		var up, down, tot uint64
		if err := rows.Scan(&logTime, &up, &down, &tot); err == nil {
			dataMap[logTime] = &GlobalHourlyTraffic{
				LogTime:  logTime,
				Upload:   up,
				Download: down,
				Total:    tot,
			}
		}
	}

	// 补齐连续 hours 小时整点平滑数据
	var list []*GlobalHourlyTraffic
	for i := hours - 1; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Hour)
		fullStr := t.Format("2006-01-02 15:00")
		labelStr := t.Format("15:00")
		if item, ok := dataMap[fullStr]; ok {
			item.LogTime = labelStr
			list = append(list, item)
		} else {
			list = append(list, &GlobalHourlyTraffic{
				LogTime:  labelStr,
				Upload:   0,
				Download: 0,
				Total:    0,
			})
		}
	}
	return list, nil
}

// GetUserDailyTraffic 获取指定用户近 N 天每日流量明细（自动补齐连续日期）
func (mysql *Mysql) GetUserDailyTraffic(userID uint, days int) ([]*DailyTraffic, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()

	if days <= 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	query := fmt.Sprintf(`
		SELECT id, user_id, username, log_date, upload, download, total
		FROM user_traffic_daily
		WHERE user_id = %d AND log_date >= '%s'
		ORDER BY log_date ASC
	`, userID, startDate)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataMap := make(map[string]*DailyTraffic)
	var userName string
	for rows.Next() {
		var id, uid uint
		var username, logDate string
		var up, down, tot uint64
		if err := rows.Scan(&id, &uid, &username, &logDate, &up, &down, &tot); err == nil {
			userName = username
			dataMap[logDate] = &DailyTraffic{
				ID:       id,
				UserID:   uid,
				Username: username,
				LogDate:  logDate,
				Upload:   up,
				Download: down,
				Total:    tot,
			}
		}
	}

	now := time.Now()
	var list []*DailyTraffic
	for i := days - 1; i >= 0; i-- {
		dStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		if item, ok := dataMap[dStr]; ok {
			list = append(list, item)
		} else {
			list = append(list, &DailyTraffic{
				ID:       0,
				UserID:   userID,
				Username: userName,
				LogDate:  dStr,
				Upload:   0,
				Download: 0,
				Total:    0,
			})
		}
	}
	return list, nil
}

// GetTodayTopUsers 获取今日流量消耗 TOP 用户
func (mysql *Mysql) GetTodayTopUsers(limit int) ([]*DailyTraffic, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()

	if limit <= 0 {
		limit = 5
	}
	today := time.Now().Format("2006-01-02")

	query := fmt.Sprintf(`
		SELECT id, user_id, username, log_date, upload, download, total
		FROM user_traffic_daily
		WHERE log_date = '%s'
		ORDER BY total DESC
		LIMIT %d
	`, today, limit)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*DailyTraffic
	for rows.Next() {
		var id, uid uint
		var username, logDate string
		var up, down, tot uint64
		if err := rows.Scan(&id, &uid, &username, &logDate, &up, &down, &tot); err == nil {
			list = append(list, &DailyTraffic{
				ID:       id,
				UserID:   uid,
				Username: username,
				LogDate:  logDate,
				Upload:   up,
				Download: down,
				Total:    tot,
			})
		}
	}
	return list, nil
}
