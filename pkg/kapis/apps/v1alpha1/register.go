package v1alpha1

import (
	"net/http"

	kapi "github.com/edgewize/edgeQ/pkg/api"
	ksappsv1alpha1 "github.com/edgewize/edgeQ/pkg/api/apps/v1alpha1"
	appsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/apiserver/query"
	"github.com/edgewize/edgeQ/pkg/apiserver/runtime"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GroupName = "apps.edgewize.io"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

func AddToContainer(container *restful.Container, cache runtimecache.Cache, client runtimeclient.Client) error {
	ws := runtime.NewWebService(GroupVersion)
	requestHandler := NewHandler(cache, client)

	ws.Route(ws.GET("/workspaces/{workspace}/infermodeltemplates").
		To(requestHandler.handleListWorkspaceInferModelTemplates).
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. sortBy=createTime")).
		Returns(http.StatusOK, constants.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/workspaces/{workspace}/infermodeltemplates/{name}").
		To(requestHandler.handleGetWorkSpaceInferModelTemplate).
		Doc("get the infer model template with the specified name").
		Returns(http.StatusOK, constants.StatusOK, appsv1alpha1.InferModelTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeltemplates").
		To(requestHandler.handleListInferModelTemplates).
		Doc("list all imtemplates").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, constants.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeltemplates/{name}").
		To(requestHandler.handleGetInferModelTemplate).
		Doc("get the apptemplate with the specified name").
		Returns(http.StatusOK, constants.StatusOK, appsv1alpha1.InferModelTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeldeployments").
		To(requestHandler.handleListAllInferModelDeployments).
		Doc("list infer model deployments in the all namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, constants.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/infermodeldeployments").
		To(requestHandler.handleListInferModelDeployments).
		Doc("list infer model deployments in the specified namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, constants.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/infermodeldeployments/{name}").
		To(requestHandler.handleGetInferModelDeployment).
		Doc("get the infer model deployment with the specified name in the specified namespace").
		Returns(http.StatusOK, constants.StatusOK, ksappsv1alpha1.InferModelDeployment{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.DELETE("/namespaces/{namespace}/infermodeldeployments/{name}").
		To(requestHandler.handleDeleteInferModelDeployment).
		Doc("get the infer model deployment with the specified name in the specified namespace").
		Returns(http.StatusOK, constants.StatusOK, ksappsv1alpha1.InferModelDeployment{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/specification/{node}").
		To(requestHandler.handleGetSpecifications).
		Doc("get NPU/GPU specifications for the node").
		Returns(http.StatusOK, constants.StatusOK, ksappsv1alpha1.NodeSpecifications{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/infermodelservers").
		To(requestHandler.handleGetDeployedInferModelServer).
		Doc("get running infer model servers").
		Param(ws.QueryParameter("node_group", "node group of edge node")).
		Returns(http.StatusOK, constants.StatusOK, ksappsv1alpha1.RunningInferModelServers{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	container.Add(ws)
	return nil
}
