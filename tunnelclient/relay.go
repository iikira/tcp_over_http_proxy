package tunnelclient

import (
	"bytes"
	"github.com/iikira/BaiduPCS-Go/pcsutil/converter"
	"net/http"
	"strconv"
	"strings"
)

// SetRelayMethod 设置允许HTTP流量中继的方法
func (thc *TunnelHTTPClient) SetRelayMethod(methods string) {
	ms := strings.Split(methods, ",")
	for k := range ms {
		ms[k] = strings.TrimSpace(ms[k])
	}
	thc.relayMethod = ms
}

func (thc *TunnelHTTPClient) isNeedRelay(data []byte) bool {
	for _, m := range thc.relayMethod {
		if bytes.HasPrefix(data, converter.ToBytes(m+" ")) {
			return true
		}
	}
	return false
}

func (thc *TunnelHTTPClient) checkRelay(data []byte) (isRelay bool, remainLength int64, newData []byte) {
	if !thc.isNeedRelay(data) {
		newData = data
		return
	}

	i := bytes.Index(data, []byte{'\r', '\n'})
	if i == -1 {
		newData = data
		return
	}

	// 是否符合3个字段
	fields := bytes.Fields(data[:i])
	if len(fields) != 3 {
		newData = data
		return
	}

	// 是否有结束字段
	endI := bytes.Index(data[i+2:], []byte{'\r', '\n', '\r', '\n'})
	if endI == -1 {
		remainLength = -1
		newData = data
		return
	}

	headers := thc.headersFunc(fields[1])

	newData = make([]byte, 0, len(data)+len(headers))
	newData = append(newData, data[:i+2]...)
	newData = append(newData, headers...)
	newData = append(newData, data[i+2:]...)
	isRelay = true
	var contentLength int64 = -1
	headerLines := bytes.Split(data[i:i+2+endI], []byte{'\r', '\n'})
	for _, line := range headerLines {
		contentLength = parseContentLength(line)
		if contentLength >= 0 {
			break
		}
	}

	// 未检测到Content-Length
	if contentLength < 0 {
		remainLength = -1
		return
	}

	remainLength = contentLength - int64(len(data)-(i+2+endI+4))
	return
}

func parseContentLength(header []byte) int64 {
	s := bytes.SplitN(header, []byte{':'}, 2)
	if len(s) != 2 {
		return -1
	}

	if strings.Compare(http.CanonicalHeaderKey(converter.ToString(s[0])), "Content-Length") != 0 {
		return -2
	}

	i, err := strconv.ParseInt(converter.ToString(bytes.TrimSpace(s[1])), 10, 64)
	if err != nil {
		return -3
	}

	return i
}
