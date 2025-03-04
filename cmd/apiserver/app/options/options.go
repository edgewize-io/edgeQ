package options

import (
	"crypto/tls"
	"fmt"
	appsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/apiserver"
	apiserverconfig "github.com/edgewize/edgeQ/pkg/apiserver/config"
	genericoptions "github.com/edgewize/edgeQ/pkg/server/options"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sync"
)

type ServerRunOptions struct {
	GenericServerRunOptions *genericoptions.ServerRunOptions
	*apiserverconfig.APIServerConfig
	SchemeOnce sync.Once
}

func NewServerRunOptions() *ServerRunOptions {
	s := &ServerRunOptions{
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		APIServerConfig:         apiserverconfig.New(),
		SchemeOnce:              sync.Once{},
	}

	return s
}

// Validate validates server run options, to find
// options' misconfiguration
func (s *ServerRunOptions) Validate() []error {
	var errors []error

	errors = append(errors, s.GenericServerRunOptions.Validate()...)
	errors = append(errors, s.KubernetesOptions.Validate()...)

	return errors
}

func (s *ServerRunOptions) NewAPIServer(stopCh <-chan struct{}) (*apiserver.APIServer, error) {
	apiServer := &apiserver.APIServer{
		Config: s.APIServerConfig,
	}

	sch := runtime.NewScheme()
	s.SchemeOnce.Do(func() {
		_ = corev1.AddToScheme(sch)
		_ = appsv1alpha1.AddToScheme(sch)
	})

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", s.GenericServerRunOptions.InsecurePort),
	}

	if s.GenericServerRunOptions.SecurePort != 0 {
		certificate, err := tls.LoadX509KeyPair(s.GenericServerRunOptions.TlsCertFile, s.GenericServerRunOptions.TlsPrivateKey)
		if err != nil {
			return nil, err
		}

		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}
		server.Addr = fmt.Sprintf(":%d", s.GenericServerRunOptions.SecurePort)
	}

	k8sConfig, err := kconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig, %s", err)
	}

	restClient, err := rest.HTTPClientFor(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest client, %s", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(k8sConfig, restClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic rest mapper, %s", err)
	}

	apiServer.RuntimeCache, err = runtimecache.New(k8sConfig, runtimecache.Options{Scheme: sch, Mapper: mapper})
	if err != nil {
		klog.Fatalf("unable to create controller runtime cache: %v", err)
	}

	apiServer.RuntimeClient, err = runtimeclient.New(k8sConfig, runtimeclient.Options{Scheme: sch, Mapper: mapper})
	if err != nil {
		klog.Fatalf("unable to create controller runtime client: %v", err)
	}
	apiServer.Server = server

	return apiServer, nil
}
