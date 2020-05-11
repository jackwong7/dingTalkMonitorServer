# dingTalkMonitorServer
Linux server CPU / memory monitoring script, over threshold automatic pin push alarm

- 本脚本为常驻内存
- 可以自由设置cpu，内存监控阀值。目前仅支持Linux系统
- 机器人设置仅支持加签，不支持ip和关键词推送
- 每当钉钉群发出通知后，会生成一条内存/CPU top10的记录到日志文件，也支持在线查看
- 日志文件达到10MB会自动清空，在线查询仅支持近100条

![推送Demo](https://github.com/jackwong7/dingTalkMonitorServer/blob/master/images/demo1.png?raw=true "推送样例")

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