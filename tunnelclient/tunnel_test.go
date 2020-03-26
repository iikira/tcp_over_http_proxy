package tunnelclient_test

import (
	"fmt"
	"golang.org/x/net/proxy"
	"log"
	"net"
	"net/url"
	"testing"
	"time"
)

func TestSocks5Tunnel(t *testing.T) {
	proxyUrl, _ := url.Parse("socks5://127.0.0.1:1099")

	var dialer proxy.Dialer
	dialer, _ = proxy.FromURL(proxyUrl, &net.Dialer{
	})

	conn, err := dialer.Dial("tcp", "101.132.157.18:80")
	if err != nil {
		log.Fatalln(err)
	}

	msg := []byte("GET / HTTP/1.1\r\nHost: 101.132.157.18\r\n\r\n")

	_, err = conn.Write(msg)
	if err != nil {
		log.Fatalln(err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(buf[:n]))

	time.Sleep(4e9)
	_, err = conn.Write(msg)
	if err != nil {
		log.Fatalln(err)
	}

	n, err = conn.Read(buf)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(buf[:n]))

	time.Sleep(8e9)
	_, err = conn.Write(msg)
	if err != nil {
		log.Fatalln(err)
	}

	n, err = conn.Read(buf)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(buf[:n]))
}
