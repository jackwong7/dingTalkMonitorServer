package handle

import (
	"net/http"
	"strings"
)

type userError string

func (e userError) Error() string {
	return e.Message()
}
func (e userError) Message() string {
	return string(e)
}

const prefix = "/log/"

var errLists = map[string]string{}
var errListKeys = []string{}

func CreateErrDetailChan() chan map[string]string {
	in := make(chan map[string]string)
	go func(errLists *map[string]string) {
		for {
			select {
			case contents := <-in:
				if len(errListKeys) > 99 {
					delete(*errLists, errListKeys[0])
					errListKeys = errListKeys[1:]
				}
				for k, v := range contents {
					errListKeys = append(errListKeys, k)
					(*errLists)[k] = v
				}
			}
		}
	}(&errLists)
	return in
}

func HandleFileList(writer http.ResponseWriter, request *http.Request) error {
	if strings.Index(request.URL.Path, prefix) != 0 {
		return userError("path must start with " + prefix)
	}
	logName := request.URL.Path[len(prefix):]
	if errLists[logName] == "" {
		writer.Write([]byte("链接已失效，请前往服务器，查看对应日志"))
	} else {
		writer.Write([]byte(errLists[logName]))
	}
	return nil
}
