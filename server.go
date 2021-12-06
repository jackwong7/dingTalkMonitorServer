package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"server/handle"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/golang/glog"
	"github.com/jackwong7/dingtalk"
	"github.com/jackwong7/ipinfo"
	telegram "github.com/jackwong7/telegrampush"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var configFileName string

//config.json
type ServerConfig struct {
	DingtalkToken   string  `json:"dingtalkToken"`
	DingtalkSecret  string  `json:"dingtalkSecret"`
	TelegramToken   string  `json:"telegramToken"`
	TelegramGroupID int64   `json:"telegramGroupId"`
	Port            string  `json:"port"`
	Filename        string  `json:"filename"`
	Interval        int     `json:"interval"`
	CPUUseRate      float64 `json:"cpuUseRate"`
	MemUsable       uint64  `json:"memUsable"`
	Ip              string  `json:"ip"`
}

var ConfigSample = `
{
	"dingtalkToken": "",
	"dingtalkSecret": "",
	"telegramToken": "",
	"telegramGroupId": 0,
	"port": "7000",
	"filename": "error.txt",
	"interval": 5,
	"cpuUseRate": 0,
	"memUsable": 2000,
	"ip":""
}
`

var (
	config      ServerConfig
	telegramBot *tgbotapi.BotAPI
	db          *sql.DB
)

//Parse config file
func parseConfig() {
	flag.StringVar(&configFileName, "config", "config.json", "配置文件,默认为./config.json")
	flag.Parse()

	_, err := os.Stat(configFileName)
	if err == nil {
		log.Println("Success to loading config!")
	}

	if os.IsNotExist(err) {
		f, err := os.Create(configFileName)
		if err != nil {
			log.Println(err.Error())
		} else {
			log.Println("The config file was generated successfully！Please restart this program")
			f.Write([]byte(ConfigSample))
			os.Exit(0)
		}
		defer f.Close()
	}

	conf, err := ioutil.ReadFile(configFileName)
	checkErr("read config file error: ", err, Error)

	var lines []string
	for _, line := range strings.Split(strings.Replace(string(conf), "\r\n", "\n", -1), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//") && line != "" {
			lines = append(lines, line)
		}
	}

	var b bytes.Buffer
	for i, line := range lines {
		if len(lines)-1 > i {
			nextLine := lines[i+1]
			if nextLine == "]" || nextLine == "]," || nextLine == "}" || nextLine == "}," {
				if strings.HasSuffix(line, ",") {
					line = strings.TrimSuffix(line, ",")
				}
			}
		}
		b.WriteString(line)
	}

	err = json.Unmarshal(b.Bytes(), &config)
	checkErr("parse config file error: ", err, Error)
}

//custom log level
const (
	Info = iota
	Warning
	Debug
	Error
)

//CheckErr checks given error
func checkErr(messge string, err error, level int) {
	if err != nil {
		switch level {
		case Info:
			color.Set(color.FgGreen)
			defer color.Unset()
			glog.Infoln(messge, err)
		case Warning, Debug:
			glog.Infoln(messge, err)
			glog.Infoln(messge, err)
		case Error:
			glog.Fatalln(messge, err)
		}
	}
}

func getDingTalkContents(msg, randstr string) string {
	return `{
	   "actionCard": {
	       "title": "服务器负载过高",
	       "text": "` + msg + `",
	       "btnOrientation": "0",
	       "btns": [
	           {
	               "title": "查看详情",
	               "actionURL": "http://` + ipJsonObj.IP + `:` + config.Port + `/log/` + randstr + `"
	           },
	       ]
	   },
	   "msgtype": "actionCard"
	}`
}

func getTelegramContents(msg, randstr string) tgbotapi.MessageConfig {
	msg = strings.ReplaceAll(msg, "###### ", "")
	msg = strings.ReplaceAll(msg, "##### ", "")
	telegramMsg := tgbotapi.NewMessage(config.TelegramGroupID, fmt.Sprintf("服务器负载过高\n%s", msg))
	var numericKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("查看详情", `http://`+ipJsonObj.IP+`:`+config.Port+`/log/`+randstr),
		),
	)
	telegramMsg.ReplyMarkup = numericKeyboard
	return telegramMsg
}
func getRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}
func check(pushdata *puthFields) {
	v, _ := mem.VirtualMemory()

	var cpupercent float64

	ps, err := cpu.Percent(time.Second, true)
	if err == nil && len(ps) > 0 {
		for _, v := range ps {
			cpupercent += v
		}
		cpupercent /= float64(len(ps))
	}
	percent := math.Ceil(cpupercent)

	memStr, cpuStr := "", ""
	if v.Total-v.Used < config.MemUsable<<20 {
		memStr = fmt.Sprintf("##### 可用内存不足 %dMB\n", config.MemUsable)
	}
	if percent > config.CPUUseRate {
		cpuStr = fmt.Sprintf("##### CPU使用率超过 %.f%%\n", config.CPUUseRate)
	}
	str := memStr + cpuStr
	if str == "" {
		return
	}
	randstr := getRandomString(10)
	str = str + "######" + fmt.Sprintf(" 警告代码: %s \n", randstr)
	writeStr := str
	if cpuStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "top -bcn1 -w512 | sed -n '8,18p' | sed '/top -b -c -n 1/d'")
		if output, err := cmd.Output(); err == nil {
			writeStr = writeStr + "Process Top10: \n  PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND\n" + string(output)
		}
	}
	if memStr != "" && cpuStr == "" {
		cmd := exec.Command("/bin/bash", "-c", "top -bcn1 -w512 | sed -n '8,18p' | sed '/top -b -c -n 1/d'")
		if output, err := cmd.Output(); err == nil {
			writeStr = writeStr + "Process Top10: \n  PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND\nProcess Rank: \n" + string(output)
		}
	}
	osWrite(writeStr, randstr)

	flushErrorToDB(db)

	str = str + fmt.Sprintf("###### 日志文件: %s \n", config.Filename)

	str = str + fmt.Sprintf("###### 总内存: %v MB, 已使用: %v MB\n"+
		"###### 内存使用率: %.f%%  CPU使用率: %.f%% \n"+
		"###### 今日提醒: %d 次, 累计提醒: %d 次\n"+
		"###### 警告发生时间: %s \n"+
		"###### 服务器名称: %s \n",
		v.Total>>20, v.Used>>20, v.UsedPercent,
		percent,
		pushdata.errorCountData.todayCount,
		pushdata.errorCountData.totalCount,
		time.Now().Format("2006-01-02 15:04:05"),
		pushdata.hostname)
	if config.DingtalkToken != "" && config.DingtalkSecret != "" {
		dingtalk.SendDingMsg(getDingTalkContents(str, randstr), config.DingtalkToken, config.DingtalkSecret)
	}
	if config.TelegramGroupID != 0 && config.TelegramToken != "" {
		telegramBot.Send(getTelegramContents(str, randstr))
	}
}

type puthFields struct {
	hostname       string
	errorCountData *errorCount
}

var ipJsonObj ipinfo.IpJson
var out chan map[string]string

func push(pushdata puthFields) {
	//如果设置了ip，就走设置的ip
	if config.Ip != "" {
		ipJsonObj.IP = config.Ip
	}
	rateLimiter := time.Tick(time.Duration(config.Interval) * time.Second)
	for {
		<-rateLimiter
		if config.Interval < 30 && time.Now().Format("05") > "05" {
			go func() {
				check(&pushdata)
			}()
		}
	}
}

func osWrite(contents, randstr string) error {
	if fileInfo, err := os.Stat(config.Filename); err == nil && fileInfo.Size()>>20 >= 10 {
		os.Truncate(config.Filename, 0)
	}

	fd, err := os.OpenFile(config.Filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	defer fd.Close()
	if err != nil {
		switch {
		case os.IsNotExist(err):
			o, err := os.Create(config.Filename)
			if err != nil {
				return err
			}
			fd = o
		default:
			return err
		}
	}

	fd_time := time.Now().Format("2006-01-02 15:04:05")
	fd_content := strings.Join([]string{"\n\n", "======", fd_time, "=====\n", contents}, "")
	m := map[string]string{randstr: fd_content}
	out <- m
	buf := []byte(fd_content)
	fd.Write(buf)
	return nil
}

type appHandle func(writer http.ResponseWriter, request *http.Request) error

type userError interface {
	error
	Message() string
}

func errWrapper(handle appHandle) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Error occurred: %v", r)
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		err := handle(writer, request)

		if err != nil {
			log.Printf("Error handling request: %s", err.Error())

			if userErr, ok := err.(userError); ok {
				http.Error(writer, userErr.Message(), http.StatusBadRequest)
				return
			}

			code := http.StatusOK
			http.Error(writer, http.StatusText(code), code)
		}
	}
}
func initDB() {
	db, _ = sql.Open("sqlite3", "./monitor.db")
	statement, _ := db.Prepare(`CREATE TABLE IF NOT EXISTS
		statistics(
		id integer primary key autoincrement,
		name varchar(20) not null,
		total_count bigint default 0,
		today_count int default 0,
		today_date int not null,
		created_at DATE )`)
	statement.Exec()
	initRecords(db)
}

func initRecords(db2 *sql.DB) {
	//q,_ := db2.Prepare("select exists (select count(1) from statistics where name = 'error_count')")
	row := db2.QueryRow("select count(1) as count from statistics where name = 'error_count'")
	var count int
	row.Scan(&count)
	if count == 0 {
		statement, _ := db2.Prepare("insert into statistics(name,today_date) values (?,?)")
		aa, _ := statement.Exec("error_count", "20210703")
		fmt.Println(aa)
	}
}

type errorCount struct {
	id         int64
	name       string
	totalCount int64
	todayCount int64
	todayDate  int64
	createdAt  string
}

var errorCountData *errorCount

func queryErrorCount(db2 *sql.DB) *errorCount {
	row := db2.QueryRow("select id,name,total_count,today_count,today_date from statistics where name = 'error_count'")
	row.Scan(&errorCountData.id, &errorCountData.name, &errorCountData.totalCount, &errorCountData.todayCount, &errorCountData.todayDate)
	return errorCountData
}
func getTodayDate() int64 {
	date, _ := strconv.ParseInt(time.Now().Format("20060102"), 10, 64)
	return date
}
func queryAndFlushTodayData(db2 *sql.DB) *errorCount {
	errorCountData = queryErrorCount(db2)
	todayDate := getTodayDate()
	if errorCountData.todayDate != todayDate {
		errorCountData.todayCount = 0
		errorCountData.todayDate = todayDate

		statement, _ := db2.Prepare("update statistics set today_count=?,today_date=?")
		statement.Exec(errorCountData.todayCount, errorCountData.todayDate)
	}
	return errorCountData
}
func flushErrorToDB(db2 *sql.DB) *errorCount {
	queryAndFlushTodayData(db2)
	statement, _ := db2.Prepare("update statistics set total_count=total_count+1,today_count=today_count+1")
	statement.Exec()
	errorCountData.totalCount = errorCountData.totalCount + 1
	errorCountData.todayCount = errorCountData.todayCount + 1
	return errorCountData
}
func main() {
	initDB()
	parseConfig()
	errorCountData = &errorCount{}

	telegramBot, _ = telegram.GetBot(config.TelegramToken)
	ipJsonObj = ipinfo.GetIp()
	out = handle.CreateErrDetailChan()
	go func() {
		http.HandleFunc("/log/", errWrapper(handle.HandleFileList))
		err := http.ListenAndServe(":"+config.Port, nil)
		if err != nil {
			panic(err)
		}
	}()

	name, _ := os.Hostname()

	pushdata := puthFields{
		hostname:       name,
		errorCountData: errorCountData,
	}

	push(pushdata)

}
