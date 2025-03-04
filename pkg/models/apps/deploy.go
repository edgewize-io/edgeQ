package apps

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/pkg/api"
	ksappsv1alpha1 "github.com/edgewize/edgeQ/pkg/api/apps/v1alpha1"
	appsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/apiserver/query"
	"github.com/edgewize/edgeQ/pkg/constants"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

type DeployOperator interface {
	ListInferModelDeployments(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error)
	GetInferModelDeployment(ctx context.Context, namespace, name string) (*ksappsv1alpha1.InferModelDeployment, error)
	DeleteInferModelDeployment(ctx context.Context, namespace string, name string, deleteWorkloads bool) (*ksappsv1alpha1.InferModelDeployment, error)
	ListNodeSpecifications(ctx context.Context, nodeName string) (*ksappsv1alpha1.NodeSpecifications, error)
	ListRunningInferModelServers(ctx context.Context, nodegroup string) (*ksappsv1alpha1.RunningInferModelServers, error)
}

type deployOperator struct {
	cache    runtimecache.Cache
	ksClient runtimeclient.Client
}

func NewDeployOperator(cache runtimecache.Cache, ksClient runtimeclient.Client) DeployOperator {
	return &deployOperator{
		cache:    cache,
		ksClient: ksClient,
	}
}

func (d *deployOperator) ListInferModelDeployments(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error) {
	deployItems, err := d.listInferModelDeployments(ctx, namespace, queryParam.Selector())
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("list imTemplates: %v", deployItems)

	res := &api.ListResult{}
	res.Items = deployItems
	res.Items = query.NewDefaultProcessor(queryParam).FilterAndSort(res.Items)
	res.TotalItems = len(res.Items)

	res.Items = query.NewDefaultProcessor(queryParam).Pagination(res.Items)
	return res, nil

}

func (d *deployOperator) GetInferModelDeployment(ctx context.Context, namespace, name string) (*ksappsv1alpha1.InferModelDeployment, error) {
	deployItem := &appsv1alpha1.InferModelDeployment{}
	err := d.cache.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployItem)
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("get InferModelDeployment: %v", deployItem)
	return d.getDeployWithWorkloadStatus(ctx, namespace, *deployItem)
}

func (d *deployOperator) DeleteInferModelDeployment(ctx context.Context, namespace string, name string, deleteWorkloads bool) (*ksappsv1alpha1.InferModelDeployment, error) {
	deployItem := &appsv1alpha1.InferModelDeployment{}
	err := d.cache.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployItem)
	if err != nil {
		return nil, err
	}

	err = d.ksClient.Delete(ctx, deployItem, &runtimeclient.DeleteOptions{})
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("delete InferModelDeployment [%v] successfully", deployItem.GetName())

	if deleteWorkloads {
		for _, selector := range deployItem.Spec.NodeSelectors {
			labelSelector := labels.Set{constants.LabelIMDeployment: name}.AsSelector()
			err = d.ksClient.DeleteAllOf(
				ctx,
				&v1.Deployment{},
				runtimeclient.InNamespace(selector.Project),
				runtimeclient.MatchingLabelsSelector{Selector: labelSelector},
			)
			if err != nil {
				return nil, err
			}
		}
	}

	result := &ksappsv1alpha1.InferModelDeployment{
		InferModelDeployment: *deployItem,
		Status:               ksappsv1alpha1.InferModelDeploymentStatus{},
	}

	return result, nil
}

func (d *deployOperator) ListNodeSpecifications(ctx context.Context, nodeName string) (*ksappsv1alpha1.NodeSpecifications, error) {
	edgeNode := &corev1.Node{}
	err := d.cache.Get(ctx, types.NamespacedName{Name: nodeName}, edgeNode)
	if err != nil {
		return nil, err
	}

	templateConfig := &corev1.ConfigMap{}
	err = d.cache.Get(ctx, types.NamespacedName{Name: edgeNode.Name, Namespace: edgeNode.Namespace}, templateConfig)
	if err != nil {
		return nil, err
	}

	devType, devModel := GetDevTypeOnNode(edgeNode.GetLabels())

	items, err := d.GetSpecificDevResTemplates(templateConfig, devType, devModel, isVirtualized(edgeNode.GetLabels()), nodeName)
	if err != nil {
		return nil, err
	}

	ret := &ksappsv1alpha1.NodeSpecifications{
		Type:              devType,
		DevModel:          devModel,
		Virtualized:       isVirtualized(edgeNode.GetLabels()),
		ResourceTemplates: items,
	}

	return ret, nil
}

func (d *deployOperator) ListRunningInferModelServers(ctx context.Context, nodegroup string) (*ksappsv1alpha1.RunningInferModelServers, error) {
	result := &ksappsv1alpha1.RunningInferModelServers{
		Servers: []ksappsv1alpha1.InferModelServer{},
	}

	if nodegroup == "" {
		return result, nil
	}

	resoureSelector := labels.NewSelector()
	requireTemplate, _ := labels.NewRequirement(constants.LabelIMTemplate, selection.Exists, nil)
	requireTemplateVersion, _ := labels.NewRequirement(constants.LabelIMTemplateVersion, selection.Exists, nil)
	requireDeployment, _ := labels.NewRequirement(constants.LabelIMDeployment, selection.Exists, nil)
	requireNodeGroup, _ := labels.NewRequirement(constants.LabelNodeGroup, selection.Equals, []string{nodegroup})
	resoureSelector = resoureSelector.Add(*requireTemplate, *requireTemplateVersion, *requireDeployment, *requireNodeGroup)

	nsList := &corev1.NamespaceList{}
	err := d.cache.List(ctx, nsList, &runtimeclient.ListOptions{})
	if err != nil {
		return nil, err
	}

	servers := []ksappsv1alpha1.InferModelServer{}
	for _, namespace := range nsList.Items {
		deployList := &appsv1alpha1.InferModelDeploymentList{}
		err = d.cache.List(ctx, deployList, &runtimeclient.ListOptions{
			LabelSelector: resoureSelector,
			Namespace:     namespace.GetName(),
		})

		if err != nil {
			klog.Errorf("list infer model server deployment in namespace [%s]", namespace.GetName(), err)
			return nil, err
		}

		for _, coreDeploy := range deployList.Items {
			serviceGroupStatus := []ksappsv1alpha1.ServiceGroupStatus{}
			imService := &appsv1alpha1.InferModelService{}
			err = d.cache.Get(ctx, types.NamespacedName{Name: coreDeploy.Name, Namespace: coreDeploy.Namespace}, imService)
			if err != nil {
				return nil, err
			}

			for _, sgItems := range imService.Status.ServiceGroups {
				serviceGroupStatus = append(serviceGroupStatus, ksappsv1alpha1.ServiceGroupStatus{
					Name:   sgItems.Name,
					IsUsed: sgItems.IsUsed,
				})
			}

			serverInstance := ksappsv1alpha1.InferModelServer{
				Name:          coreDeploy.GetName(),
				Namespace:     coreDeploy.GetNamespace(),
				ServiceGroups: serviceGroupStatus,
			}
			servers = append(servers, serverInstance)
		}
	}

	result.Servers = servers
	return result, nil
}

func (d *deployOperator) GetSpecificDevResTemplates(cm *corev1.ConfigMap, devType, devModel string, virtualized bool, nodeName string) (items []ksappsv1alpha1.TemplateItem, err error) {
	items = []ksappsv1alpha1.TemplateItem{}
	dataBytes, ok := cm.Data[constants.ResTemplateKey]
	if !ok {
		klog.Warningf("resTemplate [%s] for GPU/NPU is emtpy", cm.GetName())
		return
	}

	templateConfig := ksappsv1alpha1.ResourceTemplateConfig{}
	err = yaml.Unmarshal([]byte(dataBytes), &templateConfig)
	if err != nil {
		klog.Errorf("parse resTemplate [%s] failed", cm.GetName())
		return
	}

	items = d.ParseDevResTemplate(templateConfig, devType, devModel, virtualized, nodeName)
	return
}

func (d *deployOperator) ParseDevResTemplate(
	resTemplateConfig ksappsv1alpha1.ResourceTemplateConfig,
	devType string,
	devModel string,
	virtualized bool,
	nodeName string) (items []ksappsv1alpha1.TemplateItem) {
	items = []ksappsv1alpha1.TemplateItem{}

	var devResTemplate ksappsv1alpha1.DevResourceTemplate
	switch devType {
	case constants.EdgeNodeTypeNPU:
		devResTemplate = resTemplateConfig.NPU
	case constants.EdgeNodeTypeGPU:
		devResTemplate = resTemplateConfig.GPU
	default:
		klog.Warningf("devType only support NPU/GPU now, but it is %s", devType)
		return
	}

	var devModelDetails []ksappsv1alpha1.TemplateDetail
	var found bool
	if virtualized && devResTemplate.Virtualized != nil {
		if devModelDetails, found = devResTemplate.Virtualized[devModel]; !found {
			klog.Warningf("template not found for virtualized devType [%s] devModel [%s] ", devType, devModel)
			return
		}

		// Add Hardware info
		devModelDetails = AddHardwareInfoToVirtualTemplateDesc(devModel, devModelDetails)
	} else if !virtualized {
		var err error
		// if Physical Template not specified by admin, Get from Node Capacity
		if devResTemplate.Physical != nil {
			devModelDetails, found = devResTemplate.Physical[devModel]
			if !found {
				devModelDetails, err = d.GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel)
			}
		} else {
			devModelDetails, err = d.GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel)
		}

		if err != nil {
			klog.Errorf("get Physical template failed on node [%s] for device [%s]", nodeName, devModel)
			return
		}

	} else {
		klog.Warningf("this resTemplate for devType [%s] devModel [%s] is empty", devType, devModel)
		return
	}

	for index, template := range devModelDetails {
		item := ksappsv1alpha1.TemplateItem{
			ResourceName:    fmt.Sprintf("template-%d", index),
			Description:     template.Description,
			ResourceRequest: template.ResourceRequest,
		}

		items = append(items, item)
	}

	return
}

func (d *deployOperator) listInferModelDeployments(ctx context.Context, namespace string, selector labels.Selector) ([]interface{}, error) {
	deployList := &appsv1alpha1.InferModelDeploymentList{}
	err := d.cache.List(ctx, deployList, &runtimeclient.ListOptions{
		LabelSelector: selector,
		Namespace:     namespace,
	})

	if err != nil {
		return nil, err
	}
	var deployItems = make([]interface{}, 0)

	for _, imd := range deployList.Items {
		ret, err := d.getDeployWithWorkloadStatus(ctx, namespace, imd)
		if err != nil {
			return nil, err
		}
		deployItems = append(deployItems, ret)
	}
	return deployItems, nil
}

func (d *deployOperator) getDeployWithWorkloadStatus(ctx context.Context, namespace string, imd appsv1alpha1.InferModelDeployment) (*ksappsv1alpha1.InferModelDeployment, error) {
	ret := &ksappsv1alpha1.InferModelDeployment{
		InferModelDeployment: imd,
		Status: ksappsv1alpha1.InferModelDeploymentStatus{
			WorkloadStats: ksappsv1alpha1.WorkloadStats{},
			Workloads:     make([]*v1.Deployment, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMDeployment: imd.Name})
	deployList := &v1.DeploymentList{}
	err := d.cache.List(ctx, deployList, &runtimeclient.ListOptions{
		LabelSelector: selector,
		Namespace:     namespace,
	})

	if err != nil {
		return nil, err
	}

	if len(deployList.Items) == 0 {
		return ret, nil
	}

	for _, deployment := range deployList.Items {
		status := constants.Processing
		for _, condition := range deployment.Status.Conditions {
			switch condition.Type {
			case v1.DeploymentAvailable:
				if condition.Status == corev1.ConditionTrue {
					status = constants.Succeeded
				}
			case v1.DeploymentReplicaFailure:
				if condition.Status == corev1.ConditionTrue {
					status = constants.Failed
				}
			}
		}
		if status == constants.Failed {
			ret.Status.WorkloadStats.Failed++
		} else if status == constants.Succeeded {
			ret.Status.WorkloadStats.Succeeded++
		} else {
			ret.Status.WorkloadStats.Processing++
		}
		ret.Status.Workloads = append(ret.Status.Workloads, deployment.DeepCopy())
	}

	return ret, nil
}

func (d *deployOperator) GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel string) (entries []ksappsv1alpha1.TemplateDetail, err error) {
	entries = []ksappsv1alpha1.TemplateDetail{}
	ctx := context.Background()
	nodeInfo := &corev1.Node{}
	err = d.ksClient.Get(ctx, types.NamespacedName{Name: nodeName}, nodeInfo)
	if err != nil {
		klog.Errorf("get node %s failed, %v", nodeName, err)
		return
	}

	resourceName := GetResourceName(devModel)
	if resourceName == "" {
		err = fmt.Errorf("not support device %s now", devModel)
		return
	}

	maxChips, ok := nodeInfo.Status.Capacity[corev1.ResourceName(resourceName)]
	if !ok {
		err = fmt.Errorf("resource %s not registered on Node %s", resourceName, nodeName)
		return
	}

	maxChipCount, err := strconv.Atoi(maxChips.String())
	if err != nil {
		err = fmt.Errorf("parse resource %s:%s failed, %v", resourceName, maxChips.String(), err)
		return
	}

	for i := 1; i <= maxChipCount; i++ {
		entries = append(entries, ksappsv1alpha1.TemplateDetail{
			Description:     GetPhysicalDesc(devModel, i),
			ResourceRequest: map[string]int{resourceName: i},
		})
	}

	return
}

func GetDevTypeOnNode(nodeLabels map[string]string) (devType, devModel string) {
	devType = constants.EdgeNodeTypeCommon
	if len(nodeLabels) == 0 {
		return
	}

	npuDevModel, ok := nodeLabels[constants.EdgeNodeNPULabel]
	if ok {
		devType = constants.EdgeNodeTypeNPU
		devModel = npuDevModel
		return
	}

	gpuDevModel, ok := nodeLabels[constants.EdgeNodeGPULabel]
	if ok {
		devType = constants.EdgeNodeTypeGPU
		devModel = gpuDevModel
	}

	return
}

func AddHardwareInfoToVirtualTemplateDesc(devModel string, devModelDetails []ksappsv1alpha1.TemplateDetail) []ksappsv1alpha1.TemplateDetail {
	var dev string
	if strings.HasPrefix(devModel, constants.DeviceHuaweiPrefix) {
		dev = strings.TrimPrefix(devModel, constants.DeviceHuaweiPrefix)
	} else {
		dev = constants.DeviceNvidiaCommon
	}

	result := []ksappsv1alpha1.TemplateDetail{}
	for _, item := range devModelDetails {
		item.Description = fmt.Sprintf("%s (%s)", item.Description, dev)
		result = append(result, item)
	}

	return result
}

func isVirtualized(nodeLabels map[string]string) (virtualized bool) {
	if len(nodeLabels) == 0 {
		return
	}

	_, virtualized = nodeLabels[constants.EdgeNodeVirtualizationLabel]
	return
}

func GetResourceName(deviceName string) string {
	switch {
	case deviceName == constants.DeviceAscend310:
		return constants.ResourceAscend310
	case deviceName == constants.DeviceAscend310P:
		return constants.ResourceAscend310P
	case strings.Contains(strings.ToLower(deviceName), constants.DeviceNvidiaCommon):
		return constants.ResourceNvidia
	default:
	}

	return ""
}

func GetPhysicalDesc(devModel string, count int) string {
	var dev string
	if strings.HasPrefix(devModel, constants.DeviceHuaweiPrefix) {
		dev = strings.TrimPrefix(devModel, constants.DeviceHuaweiPrefix)
	} else {
		dev = constants.DeviceNvidiaCommon
	}

	return fmt.Sprintf("%d Chip (%s)", count, dev)
}
