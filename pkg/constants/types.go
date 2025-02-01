package constants

import "time"

var (
	AppProxyConfigFileName     = "model-mesh-proxy.yaml"
	ServerBrokerConfigFileName = "model-mesh-broker.yaml"
	ProxyServiceGroupLabel     = "client.modelmesh.edgewize.io/servicegroup"
	ServerDeployNameLabel      = "server.modelmesh.edgewize.io/name"
	ServerDeployNamespaceLabel = "server.modelmesh.edgewize.io/namespace"
	ServerBrokerHostPortLabel  = "server.modelmesh.edgewize.io/hostport"
	DefaultServiceGroup        = "default"
	ProxyServiceGroupEnv       = "SERVICE_GROUP"
	ServerBrokerEndpointEnv    = "SERVING_ENDPOINT"
	DefaultServingPort         = 5500
	DefaultBrokerHostPort      = "30550"
	DefaultMetricPort          = 5501
	DefaultInterval            = time.Minute
	ServiceGroupPrefix         = "servicegroup.edgewize.io/"
	DefaultBrokerContainerName = "model-mesh-broker"
	DefaultBrokerPortName      = "broker-grpc"
	DefaultBrokerVolumeName    = "broker-config"
	DefaultMetricsProxyPort    = "30992"
)
