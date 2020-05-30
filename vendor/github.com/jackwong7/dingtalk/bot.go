package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
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

func SendDingMsg(contents, token, secret string) error{
	//请求地址模板
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	sign := url.QueryEscape(computeHmacSha256(timestamp, secret))
	webhook := fmt.Sprintf(
		"https://oapi.dingtalk.com/robot/send?access_token=%s"+
			"&timestamp=%s"+
			"&sign=%s",
		token,
		timestamp,
		sign)
	//创建一个请求
	req, err := http.NewRequest("POST", webhook, strings.NewReader(contents))

	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	//设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	//发送请求
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
