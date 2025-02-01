package picker

import (
	"context"
	xconfig "github.com/edgewize/modelmesh/pkg/config"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/metadata"
)

type SchedulingMethod string

const (
	WeightAttributeKey = "customize_weight"

	// SP (Strict Priority—Traffic) scheduling for the selected queue and all higher queues is based strictly on the queue priority.
	RR SchedulingMethod = "RR"
	// WRR (WRR—Traffic) scheduling for the selected queue is based on WRR. The period time is divided between the WRR queues that are not empty, meaning that they have descriptors to egress. It happens only if Strict Priority queues are empty.
	WRR SchedulingMethod = "WRR"
)

var (
	pickers = map[SchedulingMethod]PickerBuilder{
		RR:  &RRPickerBuilder{},
		WRR: &WRRPickerBuilder{},
	}
)

func Builder(method string) PickerBuilder {
	if method == "" {
		return nil
	}
	builder, ok := pickers[SchedulingMethod(method)]
	if ok {
		return builder
	}
	return nil
}

// Picker is used by gRPC to pick a Resource to send an RPC.
// Balancer is expected to generate a new picker from its snapshot every time its
// internal state has changed.
//
// The pickers used by gRPC can be updated by ClientConn.UpdateState().
type Picker interface {
	// Pick returns the connection to use for this RPC and related information.
	//
	// Pick should not block.  If the balancer needs to do I/O or any blocking
	// or time-consuming work to service this call, it should return
	// ErrNoSubConnAvailable, and the Pick call will be repeated by gRPC when
	// the Picker is updated (using ClientConn.UpdateState).
	//
	// If an error is returned:
	//
	// - If the error is ErrNoSubConnAvailable, gRPC will block until a new
	//   Picker is provided by the balancer (using ClientConn.UpdateState).
	//
	// - If the error is a status error (implemented by the grpc/status
	//   package), gRPC will terminate the RPC with the code and message
	//   provided.
	//
	// - For all other errors, wait for ready RPCs will wait, but non-wait for
	//   ready RPCs will be terminated with this error's Error() string and
	//   status code Unavailable.
	Pick(info PickInfo) (PickResult, error)
}

type PickerKey = Resource

// PickerBuilder creates Picker.
type PickerBuilder interface {
	// Build returns a picker that will be used by gRPC to pick a Resource.
	Build(info *PickerBuildInfo) Picker
}

// PickerBuildInfo contains information needed by the picker builder to
// construct a picker.
type PickerBuildInfo struct {
	Resources map[Resource]ResourceInfo
}

func NewPickerBuildInfo() *PickerBuildInfo {
	return &PickerBuildInfo{
		Resources: make(map[Resource]ResourceInfo),
	}
}

func (pb *PickerBuildInfo) AddResource(r *xconfig.ServiceGroup) {
	pb.Resources[r] = ResourceInfo{
		Name:       r.Name,
		Attributes: attributes.New(WeightAttributeKey, r.Weight),
	}
}

type Resource interface {
	ResourceName() string
}

// ResourceInfo contains information about a Resource created by the base
type ResourceInfo struct {
	// ResourceName is the name of this Resource.
	Name string

	// Attributes contains arbitrary data about this Resource
	Attributes *attributes.Attributes
}

func (r *ResourceInfo) ResourceName() string {
	return r.Name
}

type PickInfo struct {
	// FullMethodName is the method name that NewClientStream() is called
	// with. The canonical format is /service/Method.
	FullMethodName string
	// Ctx is the RPC's context, and may contain relevant RPC-level information
	// like the outgoing header metadata.
	Ctx context.Context
}

// PickResult contains information related to a connection chosen for an RPC.
type PickResult struct {
	// Resource is the resource to use for this pick, if its state is Ready.
	Resource Resource

	// Done is called when the RPC is completed.  If the Resource is not ready,
	// this will be called with a nil parameter.  If the Resource is not a valid
	// type, Done may not be called.  May be nil if the balancer does not wish
	// to be notified when the RPC completes.
	Done func(DoneInfo)

	// Metadata provides a way for LB policies to inject arbitrary per-call
	// metadata. Any metadata returned here will be merged with existing
	// metadata added by the client application.
	//
	// LB policies with child policies are responsible for propagating metadata
	// injected by their children to the ClientConn, as part of Pick().
	Metadata metadata.MD
}

// DoneInfo contains additional information for done.
type DoneInfo struct {
	// Err is the rpc error the RPC finished with. It could be nil.
	Err error
	// Trailer contains the metadata from the RPC's trailer, if present.
	Trailer metadata.MD
	// BytesSent indicates if any bytes have been sent to the server.
	BytesSent bool
	// BytesReceived indicates if any byte has been received from the server.
	BytesReceived bool
	// ServerLoad is the load received from server. It's usually sent as part of
	// trailing metadata.
	//
	// The only supported type now is *orca_v3.LoadReport.
	ServerLoad interface{}
}
