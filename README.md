# dingTalkMonitorServer
Linux server CPU / memory monitoring script, over threshold automatic pin push alarm

- 本脚本为常驻内存
- 可以自由设置cpu，内存监控阀值。目前仅支持Linux系统
- 机器人设置仅支持加签，不支持ip和关键词推送
- 每当钉钉群发出通知后，会生成一条内存/CPU top10的记录到日志文件，也支持在线查看
- 日志文件达到10MB会自动清空，在线查询仅支持近100条
- V1.3更新支持配置 config.json文件, 命令参数高于文件参数 config.json
- V1.4更新支持配置 电报机器人
- V2.0历史推送统计记录到sqlite中，修改电报推送逻辑
- V2.1新增IP配置，如果配置了IP则查看详情默认使用配置的IP进行访问，如果config.json未初始化则会自动初始化一个配置文件

![推送Demo](https://github.com/jackwong7/dingTalkMonitorServer/blob/master/images/demo1.png?raw=true "推送样例")

V2.1 updated

Add Ip configure, if configure u can use configured ip to visit that.
Automatic init config.json.

V2.0 updated

History push count data record to sqlite

V1.4 updated

You can configuration telegram bot

V1.3 updated

You can configuration config.json file

like this:

```json
{
  "dingtalkToken": "input your dingtalk token",
  "dingtalkSecret": "input your dingtalk secret",
  "telegramToken": "input your telegram token",
  "telegramGroupId": "input your telegram groupId",
  "port": "7000",
  "filename": "error.txt",
  "interval": 5,
  "cpuUseRate": 50,
  "memUsable": 500,
  "ip":""
}
```

