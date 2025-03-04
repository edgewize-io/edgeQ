package apiserver

import (
	"bytes"
	"context"
	"fmt"
	apiserverconfig "github.com/edgewize/edgeQ/pkg/apiserver/config"
	"github.com/edgewize/edgeQ/pkg/utils"
	"github.com/emicklei/go-restful/v3"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	urlruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	rt "runtime"
	"time"

	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type APIServer struct {
	// number of kubesphere apiserver
	ServerCount int

	Server *http.Server

	Config *apiserverconfig.APIServerConfig

	// webservice container, where all webservice defines
	container *restful.Container

	// controller-runtime cache
	RuntimeCache runtimecache.Cache

	// controller-runtime client
	RuntimeClient runtimeclient.Client

	KubeSphereClient rest.RESTClient
}

func (s *APIServer) PrepareRun(stopCh <-chan struct{}) error {
	s.container = restful.NewContainer()
	s.container.Filter(logRequestAndResponse)
	s.container.Router(restful.CurlyRouter{})
	s.container.RecoverHandler(func(panicReason interface{}, httpWriter http.ResponseWriter) {
		logStackOnRecover(panicReason, httpWriter)
	})

	s.installKubeSphereAPIs(stopCh)
	s.installCRDAPIs()

	for _, ws := range s.container.RegisteredWebServices() {
		klog.Infof("Register %s", ws.RootPath())
		for _, r := range ws.Routes() {
			klog.Infof("    %s", r.Method+" "+r.Path)
		}
	}

	s.Server.Handler = s.container
	return nil
}

// Install all kubesphere api groups
// Installation happens before all informers start to cache objects, so
//
//	any attempt to list objects using listers will get empty results.
func (s *APIServer) installKubeSphereAPIs(stopCh <-chan struct{}) {
	urlruntime.Must(edgeclusterv1alpha1.AddToContainer(s.container, s.Config, s.RuntimeClient, s.RuntimeCache, s.K8sClient, s.K8sClient.Discovery(), s.KubeSphereClient))
	urlruntime.Must(edgeappsetv1alpha1.AddToContainer(s.container, s.RuntimeClient, s.RuntimeCache))
}

// installCRDAPIs Install CRDs to the KAPIs with List and Get options
func (s *APIServer) installCRDAPIs() {
	crds := &extv1.CustomResourceDefinitionList{}
	// TODO Maybe we need a better label name
	urlruntime.Must(s.RuntimeClient.List(context.TODO(), crds, runtimeclient.MatchingLabels{"kubesphere.io/resource-served": "true"}))
}

func (s *APIServer) Run(ctx context.Context) (err error) {
	go func() {
		if err := s.RuntimeCache.Start(ctx); err != nil {
			klog.Errorf("failed to start runtime cache: %s", err)
		}
	}()

	shutdownCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = s.Server.Shutdown(shutdownCtx)
	}()

	klog.V(0).Infof("Start listening on %s", s.Server.Addr)
	if s.Server.TLSConfig != nil {
		err = s.Server.ListenAndServeTLS("", "")
	} else {
		err = s.Server.ListenAndServe()
	}

	return err
}

func logStackOnRecover(panicReason interface{}, w http.ResponseWriter) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("recover from panic situation: - %v\r\n", panicReason))
	for i := 2; ; i += 1 {
		_, file, line, ok := rt.Caller(i)
		if !ok {
			break
		}
		buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
	}
	klog.Errorln(buffer.String())

	headers := http.Header{}
	if ct := w.Header().Get("Content-Type"); len(ct) > 0 {
		headers.Set("Accept", ct)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error"))
}

func logRequestAndResponse(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	start := time.Now()
	chain.ProcessFilter(req, resp)

	// Always log error response
	logWithVerbose := klog.V(4)
	if resp.StatusCode() > http.StatusBadRequest {
		logWithVerbose = klog.V(0)
	}

	logWithVerbose.Infof("%s - \"%s %s %s\" %d %d %dms",
		utils.RemoteIp(req.Request),
		req.Request.Method,
		req.Request.URL,
		req.Request.Proto,
		resp.StatusCode(),
		resp.ContentLength(),
		time.Since(start)/time.Millisecond,
	)
}

type errorResponder struct{}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	klog.Error(err)
	responsewriters.InternalError(w, req, err)
}
