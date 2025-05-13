package v1alpha1

import (
	xconfig "github.com/edgewize/edgeQ/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:resource:scope=Namespaced

type InferModelService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InferModelServiceSpec   `json:"spec,omitempty"`
	Status            InferModelServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InferModelServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferModelService `json:"items"`
}

type InferModelServiceSpec struct {
	BrokerConfig EdgeQosBrokerConfig `json:"brokerConfig,omitempty" yaml:"brokerConfig,omitempty" mapstructure:"brokerConfig"`
	ProxyConfig  EdgeQosProxyConfig  `json:"proxyConfig,omitempty" yaml:"proxyConfig,omitempty" mapstructure:"proxyConfig"`
	HostPort     int32               `json:"hostPort,omitempty" yaml:"hostPort,omitempty" mapstructure:"hostPort"`
}

type ServiceGroupItem struct {
	Name   string `json:"name,omitempty"`
	Client string `json:"client,omitempty"`
	IsUsed bool   `json:"isUsed,omitempty"`
}

type InferModelServiceStatus struct {
	ServiceGroups map[string]ServiceGroupItem `json:"serviceGroups,omitempty"`
}

type EdgeQosBrokerConfig struct {
	Broker        xconfig.MeshServer     `json:"broker,omitempty" yaml:"broker,omitempty" mapstructure:"broker"`
	Schedule      xconfig.Schedule       `json:"schedule,omitempty" yaml:"schedule,omitempty" mapstructure:"schedule"`
	Queue         xconfig.Queue          `json:"queue,omitempty" yaml:"queue,omitempty" mapstructure:"queue"`
	ServiceGroups []xconfig.ServiceGroup `json:"serviceGroups,omitempty" yaml:"serviceGroups,omitempty" mapstructure:"serviceGroups"`
	PromMetrics   xconfig.PromMetrics    `json:"promMetrics,omitempty" yaml:"promMetrics,omitempty" mapstructure:"promMetrics"`
	BaseOptions   xconfig.BaseConfig     `json:"baseOptions,omitempty" yaml:"baseOptions,omitempty" mapstructure:"baseOptions"`
}

type EdgeQosProxyConfig struct {
	Proxy        xconfig.MeshServer   `json:"proxy,omitempty" yaml:"proxy,omitempty" mapstructure:"proxy"`
	ServiceGroup xconfig.ServiceGroup `json:"serviceGroup,omitempty" yaml:"serviceGroup,omitempty" mapstructure:"serviceGroup"`
	BaseOptions  xconfig.BaseConfig   `json:"baseOptions,omitempty" yaml:"baseOptions,omitempty" mapstructure:"baseOptions"`
}

func init() {
	SchemeBuilder.Register(&InferModelService{}, &InferModelServiceList{})
}
