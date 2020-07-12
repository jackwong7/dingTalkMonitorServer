package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/golang/glog"
	"github.com/jackwong7/dingtalk"
	"github.com/jackwong7/ipinfo"
	"github.com/shirou/gopsutil/mem"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"server/handle"
	"strconv"
	"strings"
	"time"
)

const configFileName string = "config.json"

//config.json
type ServerConfig struct {
	Token      string  `json:"token"`
	Secret     string  `json:"secret"`
	Port       string  `json:"port"`
	Filename   string  `json:"filename"`
	Interval   int     `json:"interval"`
	CPUUseRate float64 `json:"cpuUseRate"`
	MemUsable  uint64  `json:"memUsable"`
}

var config ServerConfig

//Parse config file
func parseConfig() {
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
func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

func getCpuUsageInfo() (usage, busy, total float64) {
	idle0, total0 := getCPUSample()
	time.Sleep(3 * time.Second)
	idle1, total1 := getCPUSample()

	idleTicks := float64(idle1 - idle0)
	totalTicks := float64(total1 - total0)
	cpuUsage := 100 * (totalTicks - idleTicks) / totalTicks
	return cpuUsage, totalTicks - idleTicks, totalTicks
	//fmt.Printf("CPU usage is %f%% [busy: %f, total: %f]\n", cpuUsage, totalTicks-idleTicks, totalTicks)
}
func check(pushdata *puthFields, today *int) {
	v, _ := mem.VirtualMemory()
	percent, _, _ := getCpuUsageInfo()

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
		cmd := exec.Command("/bin/bash", "-c", "ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%cpu | head -10")
		//cmd := exec.Command("/bin/bash", "-c", "top -b -o +%CPU | head -n 17")
		if output, err := cmd.Output(); err == nil {
			writeStr = writeStr + "CPU TOP10: \n" + string(output)
		}
	}
	if memStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%mem | head -10")
		//cmd := exec.Command("/bin/bash", "-c", "top -b -o +%MEM | head -n 17")
		if output, err := cmd.Output(); err == nil {
			if cpuStr != "" {
				writeStr = writeStr + "\n"
			}
			writeStr = writeStr + "内存 TOP10: \n" + string(output)
		}
	}
	osWrite(writeStr, randstr)

	pushdata.todayRunCount++
	pushdata.allRunCount++
	if *today != time.Now().Day() {
		*today = time.Now().Day()
		pushdata.todayRunCount = 1
	}

	str = str + fmt.Sprintf("###### 日志文件: %s \n", config.Filename)

	str = str + fmt.Sprintf("###### 总内存: %v MB, 已使用: %v MB\n"+
		"###### 内存使用率: %.f%%  CPU使用率: %.f%% \n"+
		"###### 今日提醒: %d 次, 累计提醒: %d 次\n"+
		"###### 警告发生时间: %s \n"+
		"###### 服务器名称: %s \n",
		v.Total>>20, v.Used>>20, v.UsedPercent,
		percent,
		pushdata.todayRunCount,
		pushdata.allRunCount,
		time.Now().Format("2006-01-02 15:04:05"),
		pushdata.hostname)
	dingtalk.SendDingMsg(getDingTalkContents(str, randstr), config.Token, config.Secret)
}

type puthFields struct {
	hostname      string
	todayRunCount uint64
	allRunCount   uint64
}

var ipJsonObj ipinfo.IpJson
var out chan map[string]string

func push(pushdata puthFields) {
	rateLimiter := time.Tick(time.Duration(config.Interval) * time.Second)
	today := time.Now().Day()
	for {
		<-rateLimiter
		go func() {
			check(&pushdata, &today)
		}()
	}
}

func getConfig() {
	flag.StringVar(&config.Token, "token", config.Token, "机器人token必填")
	flag.StringVar(&config.Secret, "secret", config.Secret, "机器人secret必填")
	flag.StringVar(&config.Port, "port", config.Port, "出错后http查看错误日志端口,默认7000")
	flag.StringVar(&config.Filename, "file", config.Filename, "警告日志路径,默认为./error.txt")
	flag.IntVar(&config.Interval, "i", config.Interval, "脚本每多久执行一次,默认5秒")
	flag.Float64Var(&config.CPUUseRate, "cpu", config.CPUUseRate, "CPU使用率超过多少报警,默认50%")
	flag.Uint64Var(&config.MemUsable, "mem", config.MemUsable, "内存不足多少报警,默认500M")
	flag.Parse()
	if config.Token == "" {
		panic("机器人token必填")
	}
	if config.Secret == "" {
		panic("机器人secret必填")
	}
	if config.Port == "" {
		config.Port = "7000"
	}
	if config.Filename == "" {
		config.Filename = "error.txt"
	}
	if config.Interval == 0 {
		config.Interval = 5
	}
	if config.CPUUseRate == 0 {
		config.CPUUseRate = 50
	}
	if config.MemUsable == 0 {
		config.MemUsable = 500
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

func main() {
	parseConfig()
	getConfig()
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
		hostname:      name,
		todayRunCount: 0,
		allRunCount:   0,
	}

	push(pushdata)

}
