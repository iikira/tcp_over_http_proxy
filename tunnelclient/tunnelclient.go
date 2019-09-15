package tunnelclient

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type (
	ServType int

	HeadersFunc func(host []byte) string

	TunnelHTTPClient struct {
		DestAddr    string
		LocalAddr   string
		headersFunc HeadersFunc
		relayMethod []string
	}
)

const (
	SERV_HTTP_PROXY ServType = iota
	SERV_REDIRECT
)

var (
	// buf Pool
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 8192)
		},
	}
)

func NewTunnelHTTPClient() *TunnelHTTPClient {
	return &TunnelHTTPClient{}
}

func (thc *TunnelHTTPClient) SetHeadersFunc(fn HeadersFunc) {
	thc.headersFunc = fn
}

// ListenAndServe 启动服务1
func (thc *TunnelHTTPClient) ListenAndServe(st ServType) (err error) {
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
		switch st {
		case SERV_HTTP_PROXY:
			go thc.handleTunneling(conn)
		case SERV_REDIRECT:
			go thc.handleRedirect(conn)
		}
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

	thc.handle(conn, fields[1])
}

func (thc *TunnelHTTPClient) handle(conn net.Conn, connDestAddr []byte) {
	// 连接
	destConn, err := net.DialTimeout("tcp", thc.DestAddr, 10*time.Second)
	if err != nil {
		log.Printf("dial %s error: %s\n", thc.DestAddr, err)
		return
	}

	defer destConn.Close()
	destConnReader := bufio.NewReader(destConn)

	fmt.Fprintf(destConn, "CONNECT %s HTTP/1.0\r\n%s\r\n", connDestAddr, thc.headersFunc(connDestAddr))
	destFirstLine, _, err := destConnReader.ReadLine()
	if err != nil {
		log.Printf("read dest first line from %s error: %s\n", destConn.RemoteAddr(), err)
		return
	}

	destFields := bytes.Fields(destFirstLine)
	if len(destFields) < 3 {
		log.Printf("unknown dest first line: %s\n", destFields)
		return
	}

	if destFields[1][0] != '2' {
		//error
		connectStatus := bytes.Join(destFields[3:], []byte{' '})
		fmt.Fprintf(conn, "%s %s %s\r\n\r\n", connDestAddr, destFields[1], connectStatus)
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

	// 判断是否为Relay Method
	// 是的话就直连
	var (
		destConn2 net.Conn
		buf       = bufPool.Get().([]byte)
		wg        = sync.WaitGroup{}
	)
	defer bufPool.Put(buf)
	wg.Add(1)
	go func() {
		recvBuf := bufPool.Get().([]byte)
		io.CopyBuffer(conn, destConn, recvBuf)
		bufPool.Put(recvBuf)
		wg.Done()
	}()
	for {
		n, err := conn.Read(buf)
		if err != nil {
			// 结束会话
			break
		}

		isRelay, remainLength, newData := thc.checkRelay(buf[:n])
		if isRelay {
			if destConn2 == nil {
				destConn2, err = net.DialTimeout("tcp", thc.DestAddr, 10*time.Second)
				if err != nil {
					log.Printf("dial2 %s error: %s\n", thc.DestAddr, err)
					break
				}
				defer destConn2.Close()
				wg.Add(1)
				go func() {
					recvBuf := bufPool.Get().([]byte)
					io.CopyBuffer(conn, destConn2, recvBuf)
					bufPool.Put(recvBuf)
					wg.Done()
				}()
			}

			destConn2.Write(newData)
			for remainLength > 0 {
				n, err = conn.Read(buf)
				if err != nil {
					log.Printf("RELAY: read from local %s error: %s\n", conn.LocalAddr(), err)
					break
				}

				remainLength -= int64(n)
				_, err = destConn2.Write(buf[:n])
				if err != nil {
					log.Printf("RELAY: write to remote %s error: %s\n", conn.RemoteAddr(), err)
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
