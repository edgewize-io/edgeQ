package imtemplate

import (
	"context"
	apisappsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/utils"
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
		r.Recorder = mgr.GetEventRecorderFor(constants.IMTControllerName)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.IMTControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&apisappsv1alpha1.InferModelTemplate{}).
		Watches(
			&apisappsv1alpha1.InferModelTemplateVersion{},
			handler.EnqueueRequestsFromMapFunc(r.syncServiceGroup),
		).Complete(r)
}

// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeltemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeltemplateversions,verbs=get;list;watch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("received imTemplate reconcile request, %v", req)

	instance := &apisappsv1alpha1.InferModelTemplate{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMTFinalizer) {
			patch := client.MergeFrom(instance.DeepCopy())
			instance.Finalizers = append(instance.Finalizers, constants.IMTFinalizer)
			if err := r.Patch(ctx, instance, patch); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if !utils.HasString(instance.ObjectMeta.Finalizers, constants.IMTFinalizer) {
			return ctrl.Result{}, nil
		}

		klog.V(3).Infof("deleting infermodel template %s/%s", instance.Namespace, instance.Name)
		if _, err := r.undoReconcile(ctx, instance); err != nil {
			klog.Errorf("undoReconcile failed, imTemplate name %s, err: %v", instance.Name, err)
		}

		patch := client.MergeFrom(instance.DeepCopy())
		instance.Finalizers = utils.RemoveString(instance.Finalizers, constants.IMTFinalizer)
		if err := r.Patch(ctx, instance, patch); err != nil {
			klog.Errorf("failed to remove finalizer from infermodel deployment %s/%s", instance.Namespace, instance.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) syncServiceGroup(ctx context.Context, obj client.Object) (templateRequests []reconcile.Request) {
	klog.V(4).Infof("infer model template version %s updated, trigger service group updating reconcile", obj.GetName())
	templateRequests = []reconcile.Request{}
	imTemplateName, ok := obj.GetLabels()[constants.LabelIMTemplate]
	if !ok {
		klog.Warningf("failed to get template label from imTemplateVersion %s", obj.GetName())
		return
	}

	templateRequests = append(templateRequests, reconcile.Request{NamespacedName: types.NamespacedName{Name: imTemplateName}})
	return
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *apisappsv1alpha1.InferModelTemplate) (ctrl.Result, error) {
	var err error
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMTemplate: instance.Name})
	imTemplateVersions := apisappsv1alpha1.InferModelTemplateVersionList{}
	if err = r.List(ctx, &imTemplateVersions, &client.ListOptions{LabelSelector: selector}); err == nil {
		for _, item := range imTemplateVersions.Items {
			imTemplateVersion := item.DeepCopy()
			if reflect.DeepEqual(imTemplateVersion.Spec.ServiceGroup, instance.Spec.ServiceGroup) {
				continue
			}

			imTemplateVersion.Spec.ServiceGroup = instance.Spec.ServiceGroup
			err = r.Update(ctx, imTemplateVersion)
			if err != nil {
				klog.Error("sync serviceGroup from imTemplate %s to imTemplateVersion %s failed, err: %v",
					instance.GetName(), imTemplateVersion.GetName(), err)
				break
			}
		}
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) undoReconcile(ctx context.Context, instance *apisappsv1alpha1.InferModelTemplate) (ctrl.Result, error) {
	var err error
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMTemplate: instance.Name})
	imTemplateVersions := apisappsv1alpha1.InferModelTemplateList{}
	if err = r.List(ctx, &imTemplateVersions, &client.ListOptions{LabelSelector: selector}); err == nil {
		for _, item := range imTemplateVersions.Items {
			err = r.Delete(ctx, &item)
		}
	}
	return ctrl.Result{}, err
}
