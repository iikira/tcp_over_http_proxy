// +build !windows

package tunnelclient

import (
	"errors"
	"fmt"
	"github.com/iikira/BaiduPCS-Go/pcsutil/converter"
	"log"
	"net"
	"syscall"
)

var (
	ErrNoTCPConnection = errors.New("not a TCP Connection")
)

func (thc *TunnelHTTPClient) handleRedirect(c net.Conn) {
	conn, ok := c.(*net.TCPConn)
	if !ok {
		log.Printf("redirect: %s\n", ErrNoTCPConnection)
		return
	}

	// srcAddr := conn.RemoteAddr()
	dstAddr, conn, err := getOriginalDstAddr(conn)
	if err != nil {
		log.Printf("redirect: getOriginalDstAddr error: %s\n", err)
		return
	}
	defer conn.Close()

	thc.handle(conn, [][]byte{[]byte("CONNECT"), converter.ToBytes(dstAddr.String()), []byte("HTTP/1.0")})
}

func getOriginalDstAddr(conn *net.TCPConn) (addr net.Addr, c *net.TCPConn, err error) {
	defer conn.Close()

	fc, err := conn.File()
	if err != nil {
		return
	}
	defer fc.Close()

	mreq, err := syscall.GetsockoptIPv6Mreq(int(fc.Fd()), syscall.IPPROTO_IP, 80)
	if err != nil {
		return
	}

	// only ipv4 support
	ip := net.IPv4(mreq.Multiaddr[4], mreq.Multiaddr[5], mreq.Multiaddr[6], mreq.Multiaddr[7])
	port := uint16(mreq.Multiaddr[2])<<8 + uint16(mreq.Multiaddr[3])
	addr, err = net.ResolveTCPAddr("tcp4", net.JoinHostPort(ip.String(), fmt.Sprint(port)))
	if err != nil {
		return
	}

	cc, err := net.FileConn(fc)
	if err != nil {
		return
	}

	c, ok := cc.(*net.TCPConn)
	if !ok {
		err = ErrNoTCPConnection
	}
	return
}
