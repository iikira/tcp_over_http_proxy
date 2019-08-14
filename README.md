# tcp_over_http_proxy
http CONNECT tunnel

# Install
```
go get -u -v github.com/iikira/tcp_over_http_proxy
```

# Config example
```
# listen
LocalAddr="0.0.0.0:1252";
# remote HTTP proxy
DestAddr="112.2.247.193:8080";
# remote HTTP proxy custom header
Headers="Proxy-Connecton:keep-alive\r\n";
```
