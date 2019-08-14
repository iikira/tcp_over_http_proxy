package main

import (
	"flag"
	"github.com/iikira/tcp_over_http_proxy/lineconfig"
	"github.com/iikira/tcp_over_http_proxy/tunnelclient"
	"log"
)

var (
	configPath string
	lc         lineconfig.LineConfig
)

func init() {
	flag.StringVar(&configPath, "c", "tcp_over_http_proxy.conf", "config path")
	flag.Parse()

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
	tc.Headers = lc["Headers"]
	log.Fatalln(tc.ListenAndServe())
}
