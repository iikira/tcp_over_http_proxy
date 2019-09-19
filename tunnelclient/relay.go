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

	// 检测Content-Length和host
	isRelay = true
	var (
		contentLength int64 = -1
		host          []byte
		headers       string
	)
	headerLines := bytes.Split(data[i:i+2+endI], []byte{'\r', '\n'})
	for _, line := range headerLines {
		s := bytes.SplitN(line, []byte{':'}, 2)
		if len(s) != 2 {
			continue
		}

		if contentLength < 0 {
			contentLength = parseContentLength(s)
		}
		if host == nil {
			host = parseHost(s)
		}
		if host != nil && contentLength >= 0 {
			break
		}
	}

	if thc.headersFunc != nil {
		headers = thc.headersFunc(host)
		newData = make([]byte, 0, len(data)+len(headers))
		newData = append(newData, data[:i+2]...)
		newData = append(newData, headers...)
		newData = append(newData, data[i+2:]...)
	} else {
		newData = data
	}

	// 未检测到Content-Length
	if contentLength < 0 {
		remainLength = -1
		return
	}

	remainLength = contentLength - int64(len(data)-(i+2+endI+4))
	return
}

func parseContentLength(splitedHeader [][]byte) int64 {
	if strings.Compare(http.CanonicalHeaderKey(converter.ToString(splitedHeader[0])), "Content-Length") != 0 {
		return -2
	}

	i, err := strconv.ParseInt(converter.ToString(bytes.TrimSpace(splitedHeader[1])), 10, 64)
	if err != nil {
		return -3
	}

	return i
}

func parseHost(splitedHeader [][]byte) []byte {
	if strings.Compare(http.CanonicalHeaderKey(converter.ToString(splitedHeader[0])), "Host") != 0 {
		return nil
	}

	return bytes.TrimSpace(splitedHeader[1])
}
