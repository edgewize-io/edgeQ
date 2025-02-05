package imdeployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	apisappsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	xconfig "github.com/edgewize/edgeQ/pkg/config"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/utils"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

type Reconciler struct {
	client.Client
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	CurrentNamespace        string
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(constants.IMDControllerName)
	}

	if r.MaxConcurrentReconciles <= 0 {
		r.MaxConcurrentReconciles = 1
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.IMDControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		WithEventFilter(predicate.GenerationChangedPredicate{
			Funcs: predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldDeploy := e.ObjectOld.(*apisappsv1alpha1.InferModelDeployment)
					newDeploy := e.ObjectNew.(*apisappsv1alpha1.InferModelDeployment)
					return !equality.Semantic.DeepEqual(oldDeploy.Spec, newDeploy.Spec)
				},
			},
		}).
		For(&apisappsv1alpha1.InferModelDeployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodelservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=deployments/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("received imDeployment reconcile request, %v", req)

	instance := &apisappsv1alpha1.InferModelDeployment{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMDFinalizer) {
			patch := client.MergeFrom(instance.DeepCopy())
			instance.Finalizers = append(instance.Finalizers, constants.IMDFinalizer)
			if err := r.Patch(ctx, instance, patch); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMDFinalizer) {
			return ctrl.Result{}, nil
		}

		klog.V(3).Infof("deleting infermodel deployment %s/%s", instance.Namespace, instance.Name)
		patch := client.MergeFrom(instance.DeepCopy())
		instance.Finalizers = utils.RemoveString(instance.Finalizers, constants.IMDFinalizer)
		if err := r.Patch(ctx, instance, patch); err != nil {
			klog.Errorf("failed to remove finalizer from infermodel deployment %s/%s", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *apisappsv1alpha1.InferModelDeployment) (ctrl.Result, error) {
	expectedWorkloads := r.getWorkloadInfos(instance)

	err := r.deployInferModelServices(ctx, instance, expectedWorkloads)
	if err != nil {
		klog.Errorf("failed to deploy infer model services: %v", err)
		return ctrl.Result{}, err
	}

	err = r.deployWorkloadDeployments(ctx, instance, expectedWorkloads)

	// TODO 更新状态

	return ctrl.Result{}, err
}

// each inferModelService corresponds to one workload
func (r *Reconciler) deployInferModelServices(ctx context.Context, imDeploy *apisappsv1alpha1.InferModelDeployment, expectedWorkloads map[string]apisappsv1alpha1.ModelNodeSelector) (err error) {
	versions := &apisappsv1alpha1.InferModelTemplateVersionList{}
	listOpts := []client.ListOption{
		client.MatchingLabels{
			constants.LabelIMTemplate:        imDeploy.Spec.IMTemplateName,
			constants.LabelIMTemplateVersion: imDeploy.Spec.Version,
		},
	}

	err = r.Client.List(ctx, versions, listOpts...)
	if err != nil {
		return
	}

	if len(versions.Items) == 0 {
		err = fmt.Errorf("imTemplate [%s] version [%s] not found", imDeploy.Spec.IMTemplateName, imDeploy.Spec.Version)
		return
	}

	serviceList := &apisappsv1alpha1.InferModelServiceList{}
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMDeployment: imDeploy.GetName()})
	err = r.Client.List(ctx, serviceList, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     imDeploy.GetNamespace(),
	})
	if err != nil {
		klog.Errorf("failed to list infer model services: %v", err)
		return
	}

	svcCache := make(map[string]string)
	for _, service := range serviceList.Items {
		svcLabels := service.GetLabels()
		if len(svcLabels) > 0 && svcLabels[constants.LabelWorkload] != "" {
			svcCache[svcLabels[constants.LabelWorkload]] = service.GetName()
		}
	}

	// 获取基础配置
	proxyConfig, err := r.loadProxyCM(ctx)
	if err != nil {
		return
	}

	brokerConfig, err := r.loadBrokerCM(ctx, versions.Items[0])
	if err != nil {
		return
	}

	for workload, selectorInfo := range expectedWorkloads {
		if _, ok := svcCache[workload]; !ok {
			name := SafeConcatName(imDeploy.GetName(), workload)
			newService := &apisappsv1alpha1.InferModelService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: selectorInfo.Project,
					Labels: map[string]string{
						"app":                            name,
						constants.LabelWorkload:          workload,
						constants.LabelNodeGroup:         selectorInfo.NodeGroup,
						constants.LabelNode:              selectorInfo.NodeName,
						constants.LabelIMTemplate:        imDeploy.Spec.IMTemplateName,
						constants.LabelIMTemplateVersion: imDeploy.Spec.Version,
						constants.LabelIMDeployment:      imDeploy.GetName(),
						constants.LabelCreator:           constants.DefaultCreator,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: imDeploy.APIVersion,
							Kind:       imDeploy.Kind,
							Name:       imDeploy.Name,
							UID:        imDeploy.UID,
						},
					},
				},
				Spec: apisappsv1alpha1.InferModelServiceSpec{
					ProxyConfig:  proxyConfig,
					BrokerConfig: brokerConfig,
					HostPort:     selectorInfo.HostPort,
				},
				Status: apisappsv1alpha1.InferModelServiceStatus{},
			}

			err = r.Client.Create(ctx, newService)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					err = nil
				} else {
					klog.Errorf("failed to create infer model service: %v", err)
					return
				}
			}
		}
	}

	klog.V(3).Infof("deploy infer model services successfully for [%s]", imDeploy.GetName())

	// TODO 删除移除的 inferModelService

	return
}

func (r *Reconciler) deployWorkloadDeployments(ctx context.Context, imDeploy *apisappsv1alpha1.InferModelDeployment, expectedWorkloads map[string]apisappsv1alpha1.ModelNodeSelector) (err error) {
	deployList := &v1.DeploymentList{}
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMDeployment: imDeploy.GetName()})
	err = r.Client.List(ctx, deployList, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     imDeploy.GetNamespace(),
	})
	if err != nil {
		return
	}

	deployCache := make(map[string]string)
	for _, deploy := range deployList.Items {
		deployLabels := deploy.GetLabels()
		if len(deployLabels) > 0 && deployLabels[constants.LabelWorkload] != "" {
			deployCache[deployLabels[constants.LabelWorkload]] = deploy.GetName()
		}
	}

	for workload, selectorInfo := range expectedWorkloads {
		if _, ok := deployCache[workload]; !ok {
			name := SafeConcatName(imDeploy.GetName(), workload)
			var podTemplateWithCustomResRequest corev1.PodTemplateSpec
			podTemplateWithCustomResRequest, err = r.getCustomConfigToPodTemplate(imDeploy)
			newDeploy := &v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: imDeploy.GetNamespace(),
					Labels: map[string]string{
						"app":                            name,
						constants.LabelNodeGroup:         selectorInfo.NodeGroup,
						constants.LabelNode:              selectorInfo.NodeName,
						constants.LabelIMTemplate:        imDeploy.Spec.IMTemplateName,
						constants.LabelIMTemplateVersion: imDeploy.Spec.Version,
						constants.LabelIMDeployment:      imDeploy.GetName(),
						constants.LabelCreator:           constants.DefaultCreator,
					},
				},
				Spec: v1.DeploymentSpec{
					Replicas: imDeploy.Spec.DeploymentTemplate.Spec.Replicas,
					Template: podTemplateWithCustomResRequest,
					Strategy: imDeploy.Spec.DeploymentTemplate.Spec.Strategy,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":                       name,
							constants.LabelIMDeployment: imDeploy.Name,
						},
					},
				},
			}

			if selectorInfo.NodeName != "" {
				newDeploy.Spec.Template.Spec.NodeSelector = map[string]string{
					corev1.LabelHostname: selectorInfo.NodeName,
				}
			}

			r.injectAdditionalLabels(imDeploy, selectorInfo, newDeploy)

			err = r.Client.Create(ctx, newDeploy)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					err = nil
				} else {
					klog.Errorf("failed to create deployment from workload: %v", err)
					return
				}
			}
		}
	}

	// TODO 更新状态

	return
}

func (r *Reconciler) getCustomConfigToPodTemplate(imDeploy *apisappsv1alpha1.InferModelDeployment) (newPodTemplate corev1.PodTemplateSpec, err error) {
	newPodTemplate = imDeploy.Spec.DeploymentTemplate.Spec.Template
	tolerations := []corev1.Toleration{
		{
			Key:      "aicp.group/worker",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "aicp.group/resource_group",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "node-role.kubernetes.io/edge",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			// Fix https://github.com/kubeedge/kubeedge/issues/3736
			Key:      corev1.TaintNodeUnreachable,
			Operator: corev1.TolerationOpExists,
		},
	}

	for _, toleration := range newPodTemplate.Spec.Tolerations {
		tolerations = append(tolerations, toleration)
	}

	newPodTemplate.Spec.Tolerations = tolerations
	return
}

func (r *Reconciler) injectAdditionalLabels(
	instance *apisappsv1alpha1.InferModelDeployment,
	selectorInfo apisappsv1alpha1.ModelNodeSelector,
	newDeployment *v1.Deployment,
) {
	templateLabels := map[string]string{
		"app":                              newDeployment.GetName(),
		constants.LabelNodeGroup:           selectorInfo.NodeGroup,
		constants.LabelNode:                selectorInfo.NodeName,
		constants.LabelIMTemplate:          instance.Spec.IMTemplateName,
		constants.LabelIMTemplateVersion:   instance.Spec.Version,
		constants.LabelIMDeployment:        instance.Name,
		constants.WebhookBrokerInjectLabel: constants.WebhookInjectEnable,
	}

	newDeployment.Spec.Template.Labels = utils.MergeLabels(templateLabels, newDeployment.Spec.Template.Labels)
	return
}

func (r *Reconciler) loadProxyCM(ctx context.Context) (proxyConf apisappsv1alpha1.EdgeQosProxyConfig, err error) {
	baseProxyCM := &corev1.ConfigMap{}
	currNamespace, err := utils.CurrentNamespace()
	if err != nil {
		return
	}

	err = r.Client.Get(ctx, types.NamespacedName{Name: constants.ProxyConfigCM, Namespace: currNamespace}, baseProxyCM)
	if err != nil {
		return
	}

	cfgDataMap := baseProxyCM.Data
	if len(cfgDataMap) == 0 {
		err = fmt.Errorf("base proxy configMap %s data is empty", constants.ProxyConfigCM)
		return
	}

	configContent, ok := cfgDataMap[constants.EdgeQosProxyConfigName]
	if !ok {
		err = fmt.Errorf("%s is empty in base proxy configmap", constants.EdgeQosProxyConfigName)
		return
	}

	err = yaml.Unmarshal([]byte(configContent), &proxyConf)
	return
}

func (r *Reconciler) loadBrokerCM(ctx context.Context, templateVersion apisappsv1alpha1.InferModelTemplateVersion) (brokerConf apisappsv1alpha1.EdgeQosBrokerConfig, err error) {
	baseBrokerCM := &corev1.ConfigMap{}
	currNamespace, err := utils.CurrentNamespace()
	if err != nil {
		return
	}

	err = r.Client.Get(ctx, types.NamespacedName{Name: constants.BrokerConfigCM, Namespace: currNamespace}, baseBrokerCM)
	if err != nil {
		return
	}

	cfgDataMap := baseBrokerCM.Data
	if len(cfgDataMap) == 0 {
		err = fmt.Errorf("base broker configMap %s data is empty", constants.BrokerConfigCM)
		return
	}

	configContent, ok := cfgDataMap[constants.EdgeQosBrokerConfigName]
	if !ok {
		err = fmt.Errorf("%s is empty in base broker configmap", constants.EdgeQosBrokerConfigName)
		return
	}

	err = yaml.Unmarshal([]byte(configContent), &brokerConf)
	if err != nil {
		return
	}

	serviceGroups := []xconfig.ServiceGroup{}
	for name, weight := range templateVersion.Spec.ServiceGroup {
		serviceGroups = append(serviceGroups, xconfig.ServiceGroup{
			Name:   name,
			Weight: int32(weight),
		})
	}

	brokerConf.ServiceGroups = serviceGroups
	return
}

func (r *Reconciler) getWorkloadInfos(instance *apisappsv1alpha1.InferModelDeployment) map[string]apisappsv1alpha1.ModelNodeSelector {
	result := make(map[string]apisappsv1alpha1.ModelNodeSelector)
	for _, item := range instance.Spec.NodeSelectors {
		uniqueName := fmt.Sprintf("%s-%s", item.NodeGroup, item.NodeName)
		result[uniqueName] = item
	}

	return result
}

func SafeConcatName(name ...string) string {
	fullPath := strings.Join(name, "-")
	if len(fullPath) > 63 {
		digest := sha256.Sum256([]byte(fullPath))
		return strings.ReplaceAll(fullPath[0:52]+"-"+hex.EncodeToString(digest[0:])[0:10], ".-", "-")
	}
	return fullPath
}
