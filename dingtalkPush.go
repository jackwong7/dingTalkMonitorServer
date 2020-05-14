package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/shirou/gopsutil/mem"
	"gostudy/basic/req/handle"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func computeHmacSha256(message string, secret string) string {

	stringToSign := message + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func SendDingMsg(msg, randstr string) {
	//请求地址模板
	content := `{
	   "actionCard": {
	       "title": "服务器负载过高",
	       "text": "` + msg + `",
	       "btnOrientation": "0",
	       "btns": [
	           {
	               "title": "查看详情",
	               "actionURL": "http://` + ip + `:` + config.port + `/log/` + randstr + `"
	           },
	       ]
	   },
	   "msgtype": "actionCard"
	}`
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	secret := url.QueryEscape(computeHmacSha256(timestamp, "SEC49437da69b1cba497ce34bca54b91d8766c13440d3f0462bc3c5c7bb43275a5c"))
	webhook := fmt.Sprintf(
		"https://oapi.dingtalk.com/robot/send?access_token=%s"+
			"&timestamp=%s"+
			"&sign=%s",
		config.token,
		timestamp,
		secret)
	//创建一个请求
	req, err := http.NewRequest("POST", webhook, strings.NewReader(content))

	if err != nil {
		// handle error
		log.Print(err)
		return
	}

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	//设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	//发送请求
	resp, err := client.Do(req)
	if err != nil {
		// handle error
		log.Print(err)
	}
	//关闭请求
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("机器人消息发送成功，发送内容：\n%s", msg)
	} else {
		log.Printf("机器人消息发送失败，http code：%d", resp.StatusCode)
	}
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
	if v.Total-v.Used < config.memUsable<<20 {
		memStr = fmt.Sprintf("##### 可用内存不足 %dMB\n", config.memUsable)
	}
	if percent > config.cpuUseRate {
		cpuStr = fmt.Sprintf("##### CPU使用率超过 %.f%%\n", config.cpuUseRate)
	}
	str := memStr + cpuStr
	if str == "" {
		return
	}
	randstr := getRandomString(10)
	str = str + "######" + fmt.Sprintf(" 警告代码: %s \n", randstr)
	writeStr := str
	if cpuStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "ps aux|head -1;ps auxw|sort -rn -k3|head -10")
		if output, err := cmd.Output(); err == nil {
			writeStr = writeStr + "CPU TOP10: \n" + string(output)
		}
	}
	if memStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "ps aux|head -1;ps auxw|sort -rn -k4|head -10")
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

	str = str + fmt.Sprintf("###### 日志文件: %s \n", config.filename)

	str = str + fmt.Sprintf("###### 总内存: %v MB, 已使用: %v MB\n"+
		"###### 内存使用率: %.f%%  CPU使用率: %.f%% \n"+
		"###### 今日提醒: %d 次, 累计提醒: %d 次\n"+
		"###### 警告发生时间: %s \n"+
		"###### 服务器名称: %s \n",
		v.Total>>20, v.Used>>20, v.UsedPercent,
		(percent),
		pushdata.todayRunCount,
		pushdata.allRunCount,
		time.Now().Format("2006-01-02 15:04:05"),
		pushdata.hostname)
	SendDingMsg(str, randstr)
}

type puthFields struct {
	hostname      string
	todayRunCount uint64
	allRunCount   uint64
	ip            string
}

var config = getConfig()
var rateLimiter = time.Tick(time.Duration(config.interval) * time.Second)
var ip string
var out chan map[string]string

func push(pushdata puthFields) {

	today := time.Now().Day()
	for {
		<-rateLimiter
		go func() {
			check(&pushdata, &today)
		}()
	}
}

type runConfig struct {
	memUsable  uint64
	cpuUseRate float64
	interval   int
	token      string
	secret     string
	filename   string
	port       string
}

func getConfig() runConfig {
	var config runConfig
	flag.StringVar(&config.token, "token", "", "机器人token必填")
	flag.StringVar(&config.secret, "secret", "", "机器人secret必填")
	flag.StringVar(&config.port, "port", "7000", "出错后http查看错误日志端口,默认7000")
	flag.StringVar(&config.filename, "file", `error.txt`, "警告日志路径,默认为./error.txt")
	flag.IntVar(&config.interval, "i", 5, "脚本每多久执行一次,默认5秒")
	flag.Float64Var(&config.cpuUseRate, "cpu", 50, "CPU使用率超过多少报警,默认50%")
	flag.Uint64Var(&config.memUsable, "mem", 500, "内存不足多少报警,默认500M")
	flag.Parse()
	if config.token == "" {
		panic("机器人token必填")
	}
	if config.secret == "" {
		panic("机器人secret必填")
	}
	return config
}
func osWrite(contents, randstr string) error {
	if fileInfo, err := os.Stat(config.filename); err == nil && fileInfo.Size()>>20 >= 10 {
		os.Truncate(config.filename, 0)
	}

	fd, err := os.OpenFile(config.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	defer fd.Close()
	if err != nil {
		switch {
		case os.IsNotExist(err):
			o, err := os.Create(config.filename)
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

func get_external() {
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get("http://myexternalip.com/raw")
	defer resp.Body.Close()
	if err == nil {
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			ip = string(body)
		}
	}
}
func main() {
	get_external()
	out = handle.CreateErrDetailChan()
	go func() {
		http.HandleFunc("/log/", errWrapper(handle.HandleFileList))
		err := http.ListenAndServe(":"+config.port, nil)
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
