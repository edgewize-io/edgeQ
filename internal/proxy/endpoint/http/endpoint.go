package http

import (
	"context"
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
)

type HttpProxyServer struct {
	Proxy *http.Server
	Addr  string
}

func (h *HttpProxyServer) Start(_ context.Context) (err error) {
	klog.Infof("Proxy server listening on %s", h.Addr)
	return h.Proxy.ListenAndServe()
}

func (h *HttpProxyServer) Stop() {
	_ = h.Proxy.Shutdown(context.Background())
}

func GetTargetURL() (*url.URL, error) {
	targetPortEnv := os.Getenv("TARGET_PORT")
	targetPort, err := strconv.Atoi(targetPortEnv)
	if err != nil {
		return nil, err
	}

	targetAddrEnv := os.Getenv("TARGET_ADDRESS")
	if targetAddrEnv == "" {
		err = fmt.Errorf("TARGET_ADDRESS environment variable not set")
		return nil, err
	}

	return url.Parse(fmt.Sprintf("http://%s:%d", targetAddrEnv, targetPort))
}

func NewHttpProxyServer(cfg *proxyCfg.Config) (*HttpProxyServer, error) {
	targetURL, err := GetTargetURL()
	if err != nil {
		return nil, err
	}

	reverseProxy := &httputil.ReverseProxy{}
	reverseProxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Proxy.HTTP.DialTimeout, // TCP 连接超时
			KeepAlive: cfg.Proxy.HTTP.KeepAlive,   // 保持长连接时间
		}).DialContext,
		ResponseHeaderTimeout: cfg.Proxy.HTTP.HeaderTimeout,
		IdleConnTimeout:       cfg.Proxy.HTTP.IdleConnTimeout,
		MaxIdleConns:          cfg.Proxy.HTTP.MaxIdleConns,
		MaxConnsPerHost:       cfg.Proxy.HTTP.MaxConnsPerHost,
	}

	reverseProxy.Rewrite = func(request *httputil.ProxyRequest) {
		request.SetXForwarded()
		request.SetURL(targetURL)
		request.Out.Header.Add("X-Service-Group", cfg.ServiceGroup.Name)
	}

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, request *http.Request, e error) {
		klog.Warningf("request server error: %v", e)
		w.WriteHeader(http.StatusBadGateway)
	}

	proxyServer := &http.Server{
		Addr:              cfg.Proxy.Addr,
		Handler:           reverseProxy,
		ReadTimeout:       cfg.Proxy.HTTP.ReadTimeout,
		WriteTimeout:      cfg.Proxy.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.Proxy.HTTP.HeaderTimeout,
		IdleTimeout:       cfg.Proxy.HTTP.IdleConnTimeout,
	}

	return &HttpProxyServer{
		Proxy: proxyServer,
		Addr:  cfg.Proxy.Addr,
	}, nil
}
