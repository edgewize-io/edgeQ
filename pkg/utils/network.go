package utils

import (
	"net"
	"net/http"
	"strings"
)

const (
	XForwardedFor = "X-Forwarded-For"
	XRealIP       = "X-Real-IP"
	XClientIP     = "x-client-ip"
)

func ExtractNetAddress(apiAddress string) (string, string) {
	idx := strings.Index(apiAddress, "://")
	if idx < 0 {
		return "tcp", apiAddress
	}

	return apiAddress[:idx], apiAddress[idx+3:]
}

func RemoteIp(req *http.Request) string {
	remoteAddr := req.RemoteAddr
	if ip := req.Header.Get(XClientIP); ip != "" {
		remoteAddr = ip
	} else if ip := req.Header.Get(XRealIP); ip != "" {
		remoteAddr = ip
	} else if ip = req.Header.Get(XForwardedFor); ip != "" {
		remoteAddr = ip
	} else {
		remoteAddr, _, _ = net.SplitHostPort(remoteAddr)
	}

	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
	}

	return remoteAddr
}
