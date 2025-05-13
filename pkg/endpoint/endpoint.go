package endpoint

import "context"

type QosEndpoint interface {
	Start(ctx context.Context) error
	Stop()
}
