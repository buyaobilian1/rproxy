package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

var proxyHttpClient *http.Client
var settings Settings

type Settings struct {
	TargetUrl string
	ProxyUrl  string
	BindAddr  string
}

func main() {
	var (
		targetUrl = flag.String("t", "https://api.telegram.org", "target url")
		proxyUrl  = flag.String("P", "socks5://127.0.0.1:7890", "proxy url")
		localPort = flag.Int("p", 3100, "local port")
	)

	flag.Parse()

	settings = Settings{
		TargetUrl: *targetUrl,
		ProxyUrl:  *proxyUrl,
		BindAddr:  fmt.Sprintf("0.0.0.0:%d", *localPort),
	}

	proxyHttpClient = createProxyHttpClient(*proxyUrl)
	// 设置 HTTP 服务器，监听所有接口
	http.HandleFunc("/", handleRequestAndRedirect)
	log.Printf("rproxy server listen on %s.", settings.BindAddr)
	log.Printf("using proxy: %s, reverse proxy: %s => %s.\n", settings.ProxyUrl, settings.BindAddr, settings.TargetUrl)
	if err := http.ListenAndServe(settings.BindAddr, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %s", err)
	}

}

func printInfo(r *http.Request) {
	scheme := "http" // 默认为 http，如果你的服务支持 https，则需要根据情况调整
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
	info := fmt.Sprintf("request %s => %s", fullURL, settings.TargetUrl)
	log.Println(info)
}

func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	printInfo(req)
	// 创建新的请求
	targetUrl := settings.TargetUrl + req.URL.Path
	proxyReq, err := http.NewRequest(req.Method, targetUrl, req.Body)
	if err != nil {
		http.Error(res, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// 复制头部信息
	copyHeader(proxyReq.Header, req.Header)

	// 发送请求
	resp, err := proxyHttpClient.Do(proxyReq)
	if err != nil {
		http.Error(res, "Server Error", http.StatusInternalServerError)
		log.Fatalf("Error sending request : %s", err)
	}
	defer resp.Body.Close()

	// 将响应内容写回客户端
	copyHeader(res.Header(), resp.Header)
	res.WriteHeader(resp.StatusCode)
	io.Copy(res, resp.Body)
}

func createProxyHttpClient(proxyUrl string) *http.Client {
	proxy := func(_ *http.Request) (*url.URL, error) {
		return url.Parse(proxyUrl)
	}

	httpTransport := &http.Transport{
		Proxy: proxy,
	}

	return &http.Client{
		Transport: httpTransport,
		Timeout:   time.Second * 10,
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
