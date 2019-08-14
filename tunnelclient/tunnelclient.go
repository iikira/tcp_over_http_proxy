package tunnelclient

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	// "os"
	"bytes"
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
			if err == io.EOF {
				break
			}
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

	go io.Copy(destConn, connReader)
	io.Copy(conn, destConn)
}
