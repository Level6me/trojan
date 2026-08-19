package controller

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"strconv"
	"time"
	"trojan/core"
	"trojan/trojan"
)

var c *cron.Cron

// SetData 设置流量限制
func SetData(id uint, quota int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.SetQuota(id, quota); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CleanData 清空流量
func CleanData(id uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.CleanData(id); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

func monthlyResetJob() {
	mysql := core.GetMysql()
	if err := mysql.MonthlyResetData(); err != nil {
		fmt.Println("MonthlyResetError: " + err.Error())
	}
}

// GetResetDay 获取重置日
func GetResetDay() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	dayStr, _ := core.GetValue("reset_day")
	day, _ := strconv.Atoi(dayStr)
	responseBody.Data = map[string]interface{}{
		"resetDay": day,
	}
	return &responseBody
}

// UpdateResetDay 更新重置流量日
func UpdateResetDay(day uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	if day > 31 || day < 0 {
		responseBody.Msg = fmt.Sprintf("%d为非正常日期", day)
		return &responseBody
	}
	dayStr, _ := core.GetValue("reset_day")
	oldDay, _ := strconv.Atoi(dayStr)
	if day == uint(oldDay) {
		return &responseBody
	}
	if len(c.Entries()) > 1 {
		c.Remove(c.Entries()[len(c.Entries())-1].ID)
	}
	if day != 0 {
		c.AddFunc(fmt.Sprintf("0 0 %d * *", day), func() {
			monthlyResetJob()
		})
	}
	core.SetValue("reset_day", strconv.Itoa(int(day)))
	return &responseBody
}

// GetGlobalDailyTraffic 获取全站每日流量
func GetGlobalDailyTraffic(days int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	// 查询前即时计算并写入最新流量增量
	_ = mysql.RecordTrafficSnapshot()
	list, err := mysql.GetGlobalDailyTraffic(days)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	responseBody.Data = list
	return &responseBody
}

// GetGlobalHourlyTraffic 获取全站近 N 小时流量
func GetGlobalHourlyTraffic(hours int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	// 查询前即时计算并写入最新流量增量
	_ = mysql.RecordTrafficSnapshot()
	list, err := mysql.GetGlobalHourlyTraffic(hours)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	responseBody.Data = list
	return &responseBody
}

// GetUserDailyTraffic 获取指定用户每日流量
func GetUserDailyTraffic(userID uint, days int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	// 查询前即时计算并写入最新流量增量
	_ = mysql.RecordTrafficSnapshot()
	list, err := mysql.GetUserDailyTraffic(userID, days)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	responseBody.Data = list
	return &responseBody
}

// GetTodayTopUsers 获取今日流量排行榜
func GetTodayTopUsers() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	// 查询前即时计算并写入最新流量增量
	_ = mysql.RecordTrafficSnapshot()
	list, err := mysql.GetTodayTopUsers(5)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	responseBody.Data = list
	return &responseBody
}

// ScheduleTask 定时任务
func ScheduleTask() {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	c = cron.New(cron.WithLocation(loc))
	
	// 启动时立即执行一次流量快照
	go func() {
		time.Sleep(1 * time.Second)
		mysql := core.GetMysql()
		mysql.CreateTable()
		mysql.RecordTrafficSnapshot()
	}()

	// 每 30 秒高频采样一次每日流量增量
	c.AddFunc("@every 30s", func() {
		mysql := core.GetMysql()
		mysql.RecordTrafficSnapshot()
	})

	c.AddFunc("@daily", func() {
		mysql := core.GetMysql()
		if needRestart, err := mysql.DailyCheckExpire(); err != nil {
			fmt.Println("DailyCheckError: " + err.Error())
		} else if needRestart {
			trojan.Restart()
		}
	})

	dayStr, _ := core.GetValue("reset_day")
	if dayStr == "" {
		dayStr = "1"
		core.SetValue("reset_day", dayStr)
	}
	day, _ := strconv.Atoi(dayStr)
	if day != 0 {
		c.AddFunc(fmt.Sprintf("0 0 %d * *", day), func() {
			monthlyResetJob()
		})
	}
	c.Start()
}
