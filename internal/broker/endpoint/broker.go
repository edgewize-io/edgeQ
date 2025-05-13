package endpoint

import (
	"fmt"
	brokerCfg "github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/endpoint/grpc"
	"github.com/edgewize/edgeQ/internal/broker/endpoint/http"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/endpoint"
)

func GetBrokerEndpoint(cfg *brokerCfg.Config) (ep endpoint.QosEndpoint, err error) {
	switch cfg.Broker.Type {
	case constants.HTTPEndpoint:
		ep, err = http.NewHttpBrokerServer(cfg)
	case constants.GRPCEndpoint:
		ep, err = grpc.NewGRPCBrokerServer(cfg)
	default:
		err = fmt.Errorf("invalid proxy type: %s", cfg.Broker.Type)
	}

	return
}
