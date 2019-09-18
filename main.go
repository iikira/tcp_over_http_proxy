package main

import (
	"flag"
	"github.com/iikira/tcp_over_http_proxy/lineconfig"
	"github.com/iikira/tcp_over_http_proxy/tunnelclient"
	"log"
)

var (
	configPath string
	servType   tunnelclient.ServMode
	lc         lineconfig.LineConfig
)

func init() {
	flag.StringVar(&configPath, "c", "tcp_over_http_proxy.conf", "config path")
	m := flag.String("m", "http", "serve mode, http or redirect")
	flag.Parse()

	switch *m {
	case "http":
		servType = tunnelclient.SERV_HTTP_PROXY
	case "redirect":
		servType = tunnelclient.SERV_REDIRECT
	default:
		log.Fatalln("unknown serve mode")
	}
	lc = lineconfig.NewLineConfig()
	err := lc.LoadFrom(configPath)
	if err != nil {
		log.Fatalf("load config error: %s\n", err)
	}
	log.Println(lc)
}

func main() {
	tc := tunnelclient.NewTunnelHTTPClient()
	tc.LocalAddr = lc["LocalAddr"]
	tc.DestAddr = lc["DestAddr"]
	tc.SetHeadersFunc(func(firstLineFields [][]byte) string {
		return lc["Headers"]
	})
	tc.SetRelayMethod(lc["RelayMethod"])
	log.Fatalln(tc.ListenAndServe(servType))
}
