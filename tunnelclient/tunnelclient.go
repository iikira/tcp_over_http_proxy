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

	fmt.Fprintf(destConn, "%s\r\n%s\r\n", firstLine, thc.Headers)
	destReader := bufio.NewReader(destConn)
	destFirstLine, _, err := destReader.ReadLine()
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

	// 判断是否为POST
	// 是的话就直连
	var (
		destConn2 net.Conn
		wg        = sync.WaitGroup{}
	)
	wg.Add(1)
	go func() {
		io.Copy(conn, destConn)
		wg.Done()
	}()
	for {
		connLine, err := connReader.ReadSlice('\n')
		if err != nil {
			if err != bufio.ErrBufferFull {
				break
			}
			err = nil
		}
		connLineFields := bytes.Fields(connLine)
		if len(connLineFields) == 3 && bytes.Compare(connLineFields[0], []byte("POST")) == 0 {
			if destConn2 == nil {
				destConn2, err = net.DialTimeout("tcp", thc.DestAddr, 10*time.Second)
				if err != nil {
					log.Printf("dial2 %s error: %s\n", thc.DestAddr, err)
					return
				}
				defer destConn2.Close()
				wg.Add(1)
				go func() {
					io.Copy(conn, destConn2)
					wg.Done()
				}()
			}

			// POST处理
			destConn2.Write(connLine)
			destConn2.Write(converter.ToBytes(thc.Headers))
			var (
				contentLength int64 = -1
				buf                 = make([]byte, 8192)
				n             int
			)
			for {
				connLine, err = connReader.ReadSlice('\n')
				if err != nil {
					if err != bufio.ErrBufferFull {
						return
					}
				}
				destConn2.Write(connLine)
				if contentLength < 0 {
					contentLength = parseContentLength(connLine)
				}
				if bytes.Compare(connLine, []byte{'\r', '\n'}) == 0 {
					if contentLength < 0 { // 没有content-length
						break
					}
					// 准备开始传数据
					for contentLength > 0 {
						n, err = io.ReadFull(connReader, buf)
						if err != nil {
							return
						}

						destConn2.Write(buf[:n])
						contentLength -= int64(n)
					}
					break
				}
			}
			continue
		}

		// 普通处理
		destConn.Write(connLine)
		_, err = io.Copy(destConn, connReader)
		if err != nil {
			break
		}
	}
	wg.Wait()
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
