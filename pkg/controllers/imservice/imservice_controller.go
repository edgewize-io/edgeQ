package imservice

import (
	"context"
	apisappsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/utils"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler struct {
	client.Client
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(constants.IMSControllerName)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.IMSControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&apisappsv1alpha1.InferModelService{}).
		Watches(
			&v1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.reportServiceGroup),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodelservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodelservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodelservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("received imService reconcile request, %v", req)

	instance := &apisappsv1alpha1.InferModelService{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMSFinalizer) {
			patch := client.MergeFrom(instance.DeepCopy())
			instance.Finalizers = append(instance.Finalizers, constants.IMSFinalizer)
			if err := r.Patch(ctx, instance, patch); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMSFinalizer) {
			return ctrl.Result{}, nil
		}

		klog.V(3).Infof("deleting infermodel service %s/%s", instance.Namespace, instance.Name)

		patch := client.MergeFrom(instance.DeepCopy())
		instance.Finalizers = utils.RemoveString(instance.Finalizers, constants.IMSFinalizer)
		if err := r.Patch(ctx, instance, patch); err != nil {
			klog.Errorf("failed to remove finalizer from infermodel service %s/%s", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) reportServiceGroup(ctx context.Context, obj client.Object) (targetRequests []reconcile.Request) {
	klog.V(4).Infof("deployment %s updated, trigger infermodel service updating reconcile", obj.GetName())
	targetRequests = []reconcile.Request{}
	deployment := &v1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, deployment)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}

		klog.Warningf("failed to get deployment %s/%s", obj.GetNamespace(), obj.GetName())
		return
	}

	podLabels := deployment.Spec.Template.GetLabels()
	if podLabels == nil {
		return
	}

	enabled, ok := podLabels[constants.WebhookProxyInjectLabel]
	if !ok || enabled != constants.WebhookInjectEnable {
		return
	}

	serverName, ok := podLabels[constants.ServerDeployNameLabel]
	if !ok {
		return
	}

	serverNamespace, ok := podLabels[constants.ServerDeployNamespaceLabel]
	if !ok {
		return
	}

	targetRequests = append(targetRequests, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: serverNamespace, Name: serverName},
	})

	return
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *apisappsv1alpha1.InferModelService) (ctrl.Result, error) {
	// TODO update Proxy && Broker ConfigMaps

	// 更新所有的 ServiceGroup
	selector := labels.SelectorFromSet(map[string]string{
		constants.WebhookProxyInjectLabel:    constants.WebhookInjectEnable,
		constants.ServerDeployNameLabel:      instance.Name,
		constants.ServerDeployNamespaceLabel: instance.Namespace,
	})
	podList := &corev1.PodList{}
	err := r.Client.List(ctx, podList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return ctrl.Result{}, err
	}

	currServiceGroups := r.GetDesireServiceGroups(instance)

	for _, pod := range podList.Items {
		podLabels := pod.GetLabels()
		if len(podLabels) == 0 {
			continue
		}

		clientSG, ok := podLabels[constants.ProxyServiceGroupLabel]
		if !ok {
			continue
		}

		_, ok = currServiceGroups[clientSG]
		if !ok {
			continue
		}

		podRefName := utils.GetPodOwnerRefName(pod)
		currServiceGroups[clientSG] = apisappsv1alpha1.ServiceGroupItem{
			Name:   clientSG,
			IsUsed: true,
			Client: podRefName,
		}
	}

	if reflect.DeepEqual(currServiceGroups, instance.Status.ServiceGroups) {
		return ctrl.Result{}, nil
	}

	instance.Status.ServiceGroups = currServiceGroups
	err = r.Status().Update(ctx, instance)
	return ctrl.Result{}, err
}

func (r *Reconciler) GetDesireServiceGroups(instance *apisappsv1alpha1.InferModelService) (svcGroups map[string]apisappsv1alpha1.ServiceGroupItem) {
	svcGroups = make(map[string]apisappsv1alpha1.ServiceGroupItem)
	specConf := instance.Spec.BrokerConfig.ServiceGroups
	for _, group := range specConf {
		svcGroups[group.Name] = apisappsv1alpha1.ServiceGroupItem{
			Name:   group.Name,
			IsUsed: false,
			Client: "",
		}
	}

	return
}
