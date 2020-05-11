# dingTalkMonitorServer
Linux server CPU / memory monitoring script, over threshold automatic pin push alarm





## Advanced Usage
Support command
```
Usage: xxxxxx [COMMAND]

SUPPORT COMMANDS:
  -h                Get helps
  -cpu float        CPU使用率超过多少报警,默认50% (default 50)
  -file string      警告日志路径,默认为./error.txt (default "error.txt")
  -i int            脚本每多久执行一次,默认5秒 (default 5)
  -mem uint         内存不足多少报警,默认500M (default 500)
  -port string      出错后http查看错误日志端口,默认7000 (default "7000")
  -secret string    机器人secret必填
  -token string     机器人token必填
```