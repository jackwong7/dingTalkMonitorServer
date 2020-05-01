package main

import (
	"flag"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func SendDingMsg(msg string) {
	//请求地址模板
	content := `{
    "msgtype": "text", 
    "text": {
        "content": "` + msg + `"
    },
}`
	//创建一个请求
	req, err := http.NewRequest("POST", config.webHook, strings.NewReader(content))
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
		return
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
func check(pushdata *puthFields, today *int) {
	v, _ := mem.VirtualMemory()
	percent, _ := cpu.Percent(time.Second, false)

	memStr, cpuStr := "", ""
	if v.Total-v.Used < config.memUsable<<20 {
		memStr = fmt.Sprintf("可用内存不足 %dMB\n", config.memUsable)
	}
	if percent[0] > config.cpuUseRate {
		cpuStr = fmt.Sprintf("CPU使用率超过 %.f%%\n", config.cpuUseRate)
	}
	str := memStr + cpuStr
	if str == "" {
		return
	}
	str = fmt.Sprintf("错误代码: %s \n", getRandomString(10)) + str
	osWrite(str)

	pushdata.todayRunCount++
	pushdata.allRunCount++
	if *today != time.Now().Day() {
		*today = time.Now().Day()
		pushdata.todayRunCount = 1
	}

	if cpuStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "ps aux|head -1;ps auxw|sort -rn -k3|head -10|tee >> "+config.filename)
		cmd.Output()
	}
	if memStr != "" {
		cmd := exec.Command("/bin/bash", "-c", "ps aux|head -1;ps auxw|sort -rn -k4|head -10|tee >> "+config.filename)
		cmd.Output()
	}
	str = str + fmt.Sprintf("错误文件: %s \n", config.filename)

	str = str + fmt.Sprintf("总内存: %v MB, 已使用: %v MB\n"+
		"内存使用率: %.f%%  CPU使用率: %.f%% \n"+
		"今日提醒: %d 次, 累计提醒: %d 次\n"+
		"错误发生时间: %s \n"+
		"服务器名称: %s \n",
		v.Total>>20, v.Used>>20, v.UsedPercent,
		(percent[0]),
		pushdata.todayRunCount,
		pushdata.allRunCount,
		time.Now().Format("2006-01-02 15:04:05"),
		pushdata.hostname)
	//fmt.Println(str)
	SendDingMsg("机器人123：\n" + str)
}

type puthFields struct {
	hostname      string
	todayRunCount uint64
	allRunCount   uint64
	ip            string
}

var config = getConfig()
var rateLimiter = time.Tick(time.Duration(config.interval) * time.Second)

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
	webHook    string
	filename   string
}

func getConfig() runConfig {
	var config runConfig
	flag.StringVar(&config.webHook, "api", "", "Robot URL, required")
	flag.StringVar(&config.filename, "file", `error.txt`, "Error message, default to error.txt")
	flag.IntVar(&config.interval, "i", 5, "Monitoring interval, The default is 5 seconds")
	flag.Float64Var(&config.cpuUseRate, "cpu", 50, "CPU usage alarm, default 50%")
	flag.Uint64Var(&config.memUsable, "mem", 500, "Memory remaining available alarm, default 500m")
	flag.Parse()
	//log.Print(config)

	if config.webHook == "" {
		panic("Missing required, robot configuration")
	}
	return config
}
func osWrite(contents string) error {
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
	fd_content := strings.Join([]string{"\n\n", contents, "======", fd_time, "=====\n"}, "")
	buf := []byte(fd_content)
	fd.Write(buf)
	return nil
}

func main() {
	name, _ := os.Hostname()

	pushdata := puthFields{
		hostname:      name,
		todayRunCount: 0,
		allRunCount:   0,
	}

	push(pushdata)

}
