package picker

//import "google.golang.org/grpc/balancer"
//
//const WeightAttributeKey = "customize_weight"
//
//// PickerBuilder creates Picker.
//type PickerBuilder interface {
//	// Build returns a picker that will be used by gRPC to pick a Resource.
//	Build(info PickerBuildInfo) Picker
//}

//// PickerBuildInfo contains information needed by the picker builder to
//// construct a picker.
//type PickerBuildInfo struct {
//	// Resources is a map from all ready SubConns to the Addresses used to
//	// create them.
//	Resources map[PickerKey]PickerInfo
//}
//
//// PickerInfo contains information about a Resource created by the base
////
//type PickerInfo struct {
//	//Address resolver.Address // the address used to create this Resource
//	Address struct {
//		Weight int32
//	}
//}
