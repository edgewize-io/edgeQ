package constants

import "time"

var (
	ProxyConfigCM              = "edge-qos-proxy"
	BrokerConfigCM             = "edge-qos-broker"
	EdgeQosProxyConfigName     = "edge-qos-proxy.yaml"
	EdgeQosBrokerConfigName    = "edge-qos-broker.yaml"
	ProxyServiceGroupLabel     = "client.edge-qos.edgewize.io/servicegroup"
	ServerDeployNameLabel      = "server.edge-qos.edgewize.io/name"
	ServerDeployNamespaceLabel = "server.edge-qos.edgewize.io/namespace"
	DefaultServiceGroup        = "default"
	DefaultBrokerContainerPort = "15050"
	DefaultProxyContainerPort  = "15051"
	DefaultMetricPort          = 5501
	DefaultInterval            = time.Minute
	ServiceGroupPrefix         = "servicegroup.edgewize.io/"
	DefaultBrokerContainerName = "edge-qos-broker"
	DefaultBrokerPortName      = "broker-grpc"
	DefaultBrokerVolumeName    = "broker-config"
	DefaultProxyVolumeName     = "proxy-config"
	DefaultUserID              = "65532"
	DefaultUserGroupID         = "65532"
	DefaultMetricsProxyPort    = "19101"
	DefaultCreator             = "edgewize"

	LabelWorkspace              = "kubesphere.io/workspace"
	LabelIMTemplate             = "apps.edgewize.io/imtemplate"
	LabelIMTemplateVersion      = "apps.edgewize.io/imtemplateversion"
	LabelIMDeployment           = "apps.edgewize.io/imdeployment"
	LabelNodeGroup              = "apps.edgewize.io/nodegroup"
	LabelWorkload               = "apps.edgewize.io/workload"
	LabelNode                   = "apps.edgewize.io/node"
	LabelCreator                = "apps.edgewize.io/creator"
	Processing                  = "processing"
	Succeeded                   = "succeeded"
	Failed                      = "failed"
	EdgeNodeTypeNPU             = "NPU"
	EdgeNodeTypeGPU             = "GPU"
	EdgeNodeTypeCommon          = "COMMON"
	EdgeNodeNPULabel            = "accelerator"
	EdgeNodeGPULabel            = "nvidia"
	EdgeNodeVirtualizationLabel = "aicp.group/aipods_type"
	ResTemplateKey              = "resource-template.yaml"
	ResTemplateConfigMap        = "resource-template"
	ResourceAscend310           = "huawei.com/Ascend310"
	ResourceAscend310P          = "huawei.com/Ascend310P"
	ResourceNvidia              = "nvidia.com/gpu"

	DeviceAscend310    = "huawei-Ascend310"
	DeviceAscend310P   = "huawei-Ascend310P"
	DeviceNvidiaCommon = "nvidia"

	DeviceHuaweiPrefix = "huawei-"

	WebhookConfigName                  = "edge-qos-injector-webhook"
	WebhookBrokerInjectLabel           = "broker.edge-qos.edgewize.io/injection"
	WebhookProxyInjectLabel            = "proxy.edge-qos.edgewize.io/injection"
	WebhookInjectEnable                = "enabled"
	IMProxyServiceGroupLabel           = "client.edge-qos.edgewize.io/servicegroup"
	IMServerDeployNameLabel            = "server.edge-qos.edgewize.io/name"
	IMServiceGroupClientAssignedPrefix = "client.servicegroup.edgewize.io/"
	IMDefaultServiceGroup              = "default"

	IMDControllerName = "imDeployment"
	IMDFinalizer      = "imdeployment.finalizer.apps.edgewize.io"
	IMTControllerName = "imTemplate"
	IMTFinalizer      = "imTemplate.finalizer.apps.edgewize.io"
	IMSControllerName = "imService"
	IMSFinalizer      = "imService.finalizer.apps.edgewize.io"

	StatusOK = "ok"

	EdgeInferModelTemplateTag   = "EdgeInferModelTemplate"
	EdgeInferModelDeploymentTag = "EdgeInferModelDeploymentTag"

	ServiceGroupHeader  = "X-Service-Group"
	TooManyRequestsCode = 429
)
