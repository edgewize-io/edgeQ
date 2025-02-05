package picker

import (
	xconfig "github.com/edgewize/edgeQ/pkg/config"
	"google.golang.org/grpc/attributes"
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

type Picker interface {
	Pick(info PickInfo) (PickResult, error)
}

type PickerKey = Resource

// PickerBuilder creates Picker.
type PickerBuilder interface {
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

func (pb *PickerBuildInfo) AddResource(r xconfig.ServiceGroup) {
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
	Candidates []Resource
}

// PickResult contains information related to a connection chosen for an RPC.
type PickResult struct {
	// Resource is the resource to use for this pick, if its state is Ready.
	Resource Resource
}

func (pr PickResult) ResourceName() string {
	return pr.Resource.ResourceName()
}
