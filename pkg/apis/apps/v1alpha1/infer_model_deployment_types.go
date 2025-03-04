package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ModelNodeSelector struct {
	Project   string `json:"project,omitempty"`
	NodeGroup string `json:"nodeGroup,omitempty"`
	NodeName  string `json:"nodeName,omitempty"`
	// Number of port to expose on the host.
	// If specified, this must be a valid port number, 0 < x < 65536.
	// If HostNetwork is specified, this must match ContainerPort.
	// Most containers do not need this.
	HostPort        int32          `json:"hostPort,omitempty"`
	ResourceRequest map[string]int `json:"resourceRequest,omitempty"`
}

type InnerDeploymentTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the Deployment.
	// +optional
	Spec ModelDeploymentSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type InferModelDeploymentSpec struct {
	IMTemplateName     string                  `json:"imTemplateName,omitempty"`
	Version            string                  `json:"version,omitempty"`
	DeploymentTemplate InnerDeploymentTemplate `json:"deploymentTemplate,omitempty"`
	NodeSelectors      []ModelNodeSelector     `json:"nodeSelectors,omitempty"`
}

type WorkLoadStatus struct {
	Name               string      `json:"name"`
	Deployed           bool        `json:"deployed"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Message            string      `json:"message"`
}

type InferModelDeploymentStatus struct {
	WorkLoadInstances map[string]WorkLoadStatus `json:"workLoadInstances"`
}

//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +k8s:openapi-gen=true

// InferModelDeployment is the schema for the inferModelTemplates API
type InferModelDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferModelDeploymentSpec   `json:"spec,omitempty"`
	Status InferModelDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type InferModelDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferModelDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferModelDeployment{}, &InferModelDeploymentList{})
}
