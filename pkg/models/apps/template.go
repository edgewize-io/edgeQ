package apps

import (
	"context"
	"github.com/edgewize/edgeQ/pkg/api"
	ksappsv1alpha1 "github.com/edgewize/edgeQ/pkg/api/apps/v1alpha1"
	appsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/apiserver/query"
	"github.com/edgewize/edgeQ/pkg/constants"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
)

type TemplateOperator interface {
	GetInferModelTemplate(ctx context.Context, workspace, name string) (*ksappsv1alpha1.InferModelTemplate, error)
	ListInferModelTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error)
}

type templateOperator struct {
	cache    runtimecache.Cache
	ksClient client.Client
}

func NewTemplateOperator(cache runtimecache.Cache, ksClient client.Client) TemplateOperator {
	return &templateOperator{
		cache:    cache,
		ksClient: ksClient,
	}
}

func (t *templateOperator) GetInferModelTemplate(ctx context.Context, workspace, name string) (*ksappsv1alpha1.InferModelTemplate, error) {
	template := &appsv1alpha1.InferModelTemplate{}
	err := t.cache.Get(ctx, types.NamespacedName{Name: name, Namespace: workspace}, template)
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("get template: %v", template)
	return t.updateVersions(ctx, *template)
}

func (t *templateOperator) ListInferModelTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error) {
	imTemplateItems, err := t.listInferModelTemplates(ctx, workspace, queryParam.Selector())
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list imTemplates: %v", imTemplateItems)

	res := &api.ListResult{}
	res.Items = imTemplateItems
	res.Items = query.NewDefaultProcessor(queryParam).FilterAndSort(res.Items)
	res.TotalItems = len(res.Items)

	res.Items = query.NewDefaultProcessor(queryParam).Pagination(res.Items)
	return res, nil
}

func (t *templateOperator) listInferModelTemplates(ctx context.Context, workspace string, selector labels.Selector) ([]interface{}, error) {
	if workspace != "" {
		requirement, err := labels.NewRequirement(constants.LabelWorkspace, selection.Equals, []string{workspace})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*requirement)
	}

	templateList := &appsv1alpha1.InferModelTemplateList{}
	err := t.cache.List(ctx, templateList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list imTemplates: %v, size: %d", templateList.Items, len(templateList.Items))

	resItems := make([]interface{}, 0)
	for _, template := range templateList.Items {
		ret, err := t.updateVersions(ctx, template)
		if err != nil {
			return nil, err
		}
		resItems = append(resItems, ret)
	}
	return resItems, nil
}

func (t *templateOperator) updateVersions(ctx context.Context, template appsv1alpha1.InferModelTemplate) (*ksappsv1alpha1.InferModelTemplate, error) {
	ret := &ksappsv1alpha1.InferModelTemplate{
		InferModelTemplate: template,
		Spec: ksappsv1alpha1.InferModelTemplateSpec{
			LatestVersion: "",
			ServiceGroup:  template.Spec.ServiceGroup,
			VersionList:   make([]appsv1alpha1.InferModelTemplateVersion, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{constants.LabelIMTemplate: template.Name})
	klog.V(4).Infof("list imTemplateVersions: %v", selector)

	templateList := &appsv1alpha1.InferModelTemplateVersionList{}
	err := t.cache.List(ctx, templateList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(templateList.Items) == 0 {
		return ret, nil
	}

	sort.Slice(templateList.Items, func(i, j int) bool {
		return templateList.Items[j].CreationTimestamp.Before(&templateList.Items[i].CreationTimestamp)
	})
	ret.Spec.LatestVersion = templateList.Items[0].Labels[constants.LabelIMTemplateVersion]
	ret.Spec.VersionList = templateList.Items
	klog.V(4).Infof("fill inferModelTemplateVersions: %v", ret)
	return ret, nil
}
