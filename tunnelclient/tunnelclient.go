package tunnelclient

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/iikira/BaiduPCS-Go/pcsutil/converter"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	TunnelHTTPClient struct {
		DestAddr  string
		LocalAddr string
		Headers   string
	}
)

func NewTunnelHTTPClient() *TunnelHTTPClient {
	return &TunnelHTTPClient{}
}

// ListenAndServe 启动服务1
func (thc *TunnelHTTPClient) ListenAndServe() (err error) {
	listener, err := net.Listen("tcp", thc.LocalAddr)
	if err != nil {
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %s\n", err)
			continue
		}
		go thc.handleTunneling(conn)
	}
}

func (thc *TunnelHTTPClient) handleTunneling(conn net.Conn) {
	defer conn.Close()

	connReader := bufio.NewReader(conn)
	firstLine, _, err := connReader.ReadLine()
	if err != nil {
		log.Printf("read first line from %s error: %s\n", conn.RemoteAddr(), err)
		return
	}

	// 解析首行
	fields := bytes.Fields(firstLine)
	if len(fields) != 3 {
		log.Printf("unknown first line: %s\n", firstLine)
		return
	}

	if bytes.Compare(fields[0], []byte("CONNECT")) != 0 {
		log.Printf("unknown method: %s\n", fields[0])
		return
	}

	fmt.Fprintf(conn, "%s 200 Connection established\r\nConnection: keep-alive\r\n\r\n", fields[2])

	for { // 读取剩下的数据
		line, _, err := connReader.ReadLine()
		if err != nil {
			log.Printf("read from %s error: %s\n", conn.RemoteAddr(), err)
			return
		}
		if len(line) == 0 { // 读取完毕
			break
		}
	}

	// 连接
	destConn, err := net.DialTimeout("tcp", thc.DestAddr, 10*time.Second)
	if err != nil {
		log.Printf("dial %s error: %s\n", thc.DestAddr, err)
		return
	}

	defer destConn.Close()
	destConnReader := bufio.NewReader(destConn)

	fmt.Fprintf(destConn, "%s\r\n%s\r\n", firstLine, thc.Headers)
	destFirstLine, _, err := destConnReader.ReadLine()
	if err != nil {
		log.Printf("read dest first line from %s error: %s\n", destConn.RemoteAddr(), err)
		return
	}

	destFields := bytes.Fields(destFirstLine)
	if len(fields) != 3 {
		log.Printf("unknown dest first line: %s\n", firstLine)
		return
	}

	if destFields[1][0] != '2' {
		//error
		fmt.Fprintf(conn, "%s %s %s\r\n\r\n", fields[2], destFields[1], destFields[2])
		return
	}

	for { // 读取destConn剩下的数据
		line, _, err := destConnReader.ReadLine()
		if err != nil {
			log.Printf("read from %s error: %s\n", destConn.RemoteAddr(), err)
			return
		}
		if len(line) == 0 { // 读取完毕
			break
		}
	}

	// 判断是否为POST
	// 是的话就直连
	var (
		destConn2 net.Conn
		buf       = make([]byte, 8192)
		wg        = sync.WaitGroup{}
	)
	wg.Add(1)
	go func() {
		io.Copy(conn, destConn)
		wg.Done()
	}()
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("read from local %s error: %s\n", conn.LocalAddr(), err)
			break
		}

		isPost, remainLength, newData := thc.checkPOST(buf[:n])
		if isPost {
			if destConn2 == nil {
				destConn2, err = net.DialTimeout("tcp", thc.DestAddr, 10*time.Second)
				if err != nil {
					log.Printf("dial2 %s error: %s\n", thc.DestAddr, err)
					break
				}
				defer destConn2.Close()
				wg.Add(1)
				go func() {
					io.Copy(conn, destConn2)
					wg.Done()
				}()
			}

			destConn2.Write(newData)
			for remainLength > 0 {
				n, err = conn.Read(buf)
				if err != nil {
					log.Printf("POST: read from local %s error: %s\n", conn.LocalAddr(), err)
					break
				}

				remainLength -= int64(n)
				_, err = destConn2.Write(buf[:n])
				if err != nil {
					log.Printf("POST: write to remote %s error: %s\n", conn.RemoteAddr(), err)
					break
				}
			}
			continue
		}

		// 普通处理
		destConn.Write(buf[:n])
	}
	wg.Wait()
}

func (thc *TunnelHTTPClient) checkPOST(data []byte) (ok bool, remainLength int64, newData []byte) {
	if !bytes.HasPrefix(data, []byte("POST ")) {
		newData = data
		return
	}

	i := bytes.Index(data, []byte{'\r', '\n'})
	if i == -1 {
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

	newData = make([]byte, 0, len(data)+len(thc.Headers))
	newData = append(newData, data[:i+2]...)
	newData = append(newData, thc.Headers...)
	newData = append(newData, data[i+2:]...)
	ok = true
	var contentLength int64 = -1
	headerLines := bytes.Split(data[i:endI], []byte{'\r', '\n'})
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
