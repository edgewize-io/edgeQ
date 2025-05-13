package endpoint

import (
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	"github.com/edgewize/edgeQ/internal/proxy/endpoint/grpc"
	"github.com/edgewize/edgeQ/internal/proxy/endpoint/http"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/endpoint"
)

func GetProxyEndpoint(cfg *proxyCfg.Config) (ep endpoint.QosEndpoint, err error) {
	switch cfg.Proxy.Type {
	case constants.HTTPEndpoint:
		ep, err = http.NewHttpProxyServer(cfg)
	case constants.GRPCEndpoint:
		ep, err = grpc.NewGRPCProxyServer(cfg)
	default:
		err = fmt.Errorf("invalid proxy type: %s", cfg.Proxy.Type)
	}

	return
}
