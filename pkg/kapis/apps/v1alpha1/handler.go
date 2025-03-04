package v1alpha1

import (
	kapi "github.com/edgewize/edgeQ/pkg/api"
	"github.com/edgewize/edgeQ/pkg/apiserver/query"
	"github.com/edgewize/edgeQ/pkg/models/apps"
	"github.com/emicklei/go-restful"
	"k8s.io/klog/v2"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type Handler struct {
	templateOperator apps.TemplateOperator
	deployOperator   apps.DeployOperator
}

func NewHandler(cache runtimecache.Cache, client runtimeclient.Client) *Handler {
	return &Handler{
		templateOperator: apps.NewTemplateOperator(cache, client),
		deployOperator:   apps.NewDeployOperator(cache, client),
	}
}

func (h *Handler) handleListWorkspaceInferModelTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)
	workspace := req.PathParameter("workspace")

	result, err := h.templateOperator.ListInferModelTemplate(req.Request.Context(), workspace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleGetWorkSpaceInferModelTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	workspace := req.PathParameter("workspace")

	result, err := h.templateOperator.GetInferModelTemplate(req.Request.Context(), workspace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleListInferModelTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.templateOperator.ListInferModelTemplate(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleGetInferModelTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")

	result, err := h.templateOperator.GetInferModelTemplate(req.Request.Context(), "", name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleListAllInferModelDeployments(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.deployOperator.ListInferModelDeployments(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleListInferModelDeployments(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	queryParams := query.ParseQueryParameter(req)

	result, err := h.deployOperator.ListInferModelDeployments(req.Request.Context(), namespace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleGetInferModelDeployment(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")

	result, err := h.deployOperator.GetInferModelDeployment(req.Request.Context(), namespace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleDeleteInferModelDeployment(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	deleteWorkloads, _ := strconv.ParseBool(req.QueryParameter("delete_workloads"))

	result, err := h.deployOperator.DeleteInferModelDeployment(req.Request.Context(), namespace, name, deleteWorkloads)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *Handler) handleGetSpecifications(req *restful.Request, resp *restful.Response) {
	nodeName := req.PathParameter("node")
	result, err := h.deployOperator.ListNodeSpecifications(req.Request.Context(), nodeName)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}

	resp.WriteEntity(result)
}

func (h *Handler) handleGetDeployedInferModelServer(req *restful.Request, resp *restful.Response) {
	nodeGroup := req.QueryParameter("node_group")

	result, err := h.deployOperator.ListRunningInferModelServers(req.Request.Context(), nodeGroup)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}

	resp.WriteEntity(result)
}
