package tunnelclient

import (
	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/server"
	"github.com/iikira/BaiduPCS-Go/pcsutil/converter"
	"log"
	"net"
)

func (thc *TunnelHTTPClient) handleSocks5(conn net.Conn) {
	conn = gosocks5.ServerConn(conn, server.DefaultSelector)
	req, err := gosocks5.ReadRequest(conn)
	if err != nil {
		log.Printf("socks5: read request error: %s\n", err)
		conn.Close()
		return
	}

	switch req.Cmd {
	case gosocks5.CmdConnect:
		thc.handleSocks5Connect(conn, req)

	case gosocks5.CmdBind:
		fallthrough
	case gosocks5.CmdUdp:
		fallthrough
	default:
		log.Printf("%d: unsupported command", gosocks5.CmdUnsupported)
		conn.Close()
		return
	}
}

func (thc *TunnelHTTPClient) handleSocks5Connect(conn net.Conn, req *gosocks5.Request) {
	defer conn.Close()
	rep := gosocks5.NewReply(gosocks5.Succeeded, nil)
	if err := rep.Write(conn); err != nil {
		log.Printf("socks5 reply error: %s\n", err)
		return
	}

	thc.handle(conn, converter.ToBytes(req.Addr.String()))
}
