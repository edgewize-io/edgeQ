package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	brokerCfg "github.com/edgewize/edgeQ/internal/broker/config"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	"github.com/edgewize/edgeQ/pkg/client/clientset/versioned"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/utils"
	"gopkg.in/yaml.v2"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

var (
	WebhookProxyInjectPath              = "/proxy"
	WebHookBrokerInjectPath             = "/broker"
	admissionWebhookAnnotationInjectKey = "sidecar-injector-webhook.edgewize.io/inject"
	admissionWebhookAnnotationStatusKey = "sidecar-injector-webhook.edgewize.io/status"
	runtimeScheme                       = runtime.NewScheme()
	codecs                              = serializer.NewCodecFactory(runtimeScheme)
	deserializer                        = codecs.UniversalDeserializer()
	ignoredNamespaces                   = []string{
		metav1.NamespaceSystem,
		metav1.NamespacePublic,
	}
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type WebhookServer struct {
	Server           *http.Server
	ProxySidecar     *SidecarConfig
	BrokerSidecar    *SidecarConfig
	K8sClientset     *k8s.Clientset
	CrdClientSet     *versioned.Clientset
	CurrentNamespace string
}

type SidecarConfig struct {
	InitContainers []corev1.Container `yaml:"initContainers"`
	Containers     []corev1.Container `yaml:"containers"`
	Volumes        []corev1.Volume    `yaml:"volumes"`
}

func NewWebHookServer(name string, namespace string, port int32, proxyConf *SidecarConfig, brokerConf *SidecarConfig) (ws *WebhookServer, err error) {
	dnsNames := []string{
		name,
		fmt.Sprintf("%s.%s", name, namespace),
		fmt.Sprintf("%s.%s.svc", name, namespace),
		fmt.Sprintf("%s.%s.svc.cluster", name, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace),
	}

	commonName := name + "." + namespace + "." + "svc"

	org := "edgewize"
	caPEM, certPEM, certKeyPEM, err := GenerateCert([]string{org}, dnsNames, commonName)
	if err != nil {
		klog.Errorf("Failed to generate ca and certificate key pair: %v", err)
		return
	}

	pair, err := tls.X509KeyPair(certPEM.Bytes(), certKeyPEM.Bytes())
	if err != nil {
		klog.Errorf("Failed to load certificate key pair: %v", err)
		return
	}

	klog.Infof("Initializing the kube client...")
	kubeconfig := os.Getenv("KUBECONFIG")
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return
	}

	k8sClientset, err := k8s.NewForConfig(conf)
	if err != nil {
		return
	}

	crdClientset, err := versioned.NewForConfig(conf)
	if err != nil {
		return
	}

	err = CreateOrUpdateMutatingWebhookConfiguration(k8sClientset, caPEM, name, namespace, port)
	if err != nil {
		klog.Errorf("Failed to create or update the mutating webhook configuration: %v", err)
		return
	}

	ws = &WebhookServer{
		Server: &http.Server{
			Addr:      fmt.Sprintf("0.0.0.0:%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
		ProxySidecar:     proxyConf,
		BrokerSidecar:    brokerConf,
		K8sClientset:     k8sClientset,
		CrdClientSet:     crdClientset,
		CurrentNamespace: namespace,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(WebhookProxyInjectPath, ws.InjectProxy)
	mux.HandleFunc(WebHookBrokerInjectPath, ws.InjectBroker)
	readyFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}

	mux.HandleFunc("/healthz", readyFunc)
	mux.HandleFunc("/readyz", readyFunc)
	ws.Server.Handler = mux

	return
}

func (ws *WebhookServer) Serve() error {
	return ws.Server.ListenAndServeTLS("", "")
}

func (ws *WebhookServer) Stop() error {
	return ws.Server.Shutdown(context.Background())
}

func (sc *SidecarConfig) DeepCopy() *SidecarConfig {
	newSidecarConfig := &SidecarConfig{
		InitContainers: []corev1.Container{},
		Containers:     []corev1.Container{},
		Volumes:        []corev1.Volume{},
	}

	for _, initContainer := range sc.InitContainers {
		newInitContainer := (&initContainer).DeepCopy()
		if newInitContainer != nil {
			newSidecarConfig.Containers = append(newSidecarConfig.InitContainers, *newInitContainer)
		}
	}

	for _, container := range sc.Containers {
		newContainer := (&container).DeepCopy()
		if newContainer != nil {
			newSidecarConfig.Containers = append(newSidecarConfig.Containers, *newContainer)
		}
	}

	for _, volume := range sc.Volumes {
		newVolume := (&volume).DeepCopy()
		if newVolume != nil {
			newSidecarConfig.Volumes = append(newSidecarConfig.Volumes, *newVolume)
		}
	}

	return newSidecarConfig
}

func (sc *SidecarConfig) AddProxyIptablesRule(serverAddr string, serverPort int32) {
	if len(sc.InitContainers) == 0 {
		return
	}

	initContainer := sc.InitContainers[0]
	initContainer.Command = []string{"/bin/bash", "-c"}
	initContainer.Args = []string{
		fmt.Sprintf("iptables -t nat -A OUTPUT -p tcp -d %s --dport %d -m owner ! --uid-owner %s -j DNAT --to-destination 127.0.0.1:%s",
			serverAddr, serverPort, constants.DefaultUserID, constants.DefaultProxyContainerPort),
	}

	sc.InitContainers = []corev1.Container{initContainer}
}

func (sc *SidecarConfig) AddBrokerIptablesRule(serverContainerPort int32) {
	if len(sc.InitContainers) == 0 {
		return
	}

	initContainer := sc.InitContainers[0]
	initContainer.Command = []string{"/bin/bash", "-c"}
	initContainer.Args = []string{
		fmt.Sprintf("iptables -t nat -A PREROUTING -p tcp  --dport %d -j REDIRECT --to-port %d;",
			serverContainerPort, constants.DefaultBrokerContainerPort),
	}

	sc.InitContainers = []corev1.Container{initContainer}
}

func (sc *SidecarConfig) RenameVolumeRefCM(cm, volumeName string) {
	newVolumes := []corev1.Volume{}
	for _, volume := range sc.Volumes {
		if volume.Name == volumeName {
			volume.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm,
				},
			}
		}
		newVolumes = append(newVolumes, volume)
	}
	sc.Volumes = newVolumes
}

func (whs *WebhookServer) inject(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Warning("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Warningf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Warningf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whs.mutate(&ar)
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Warningf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Info("Ready to write response ...")
	if _, err := w.Write(resp); err != nil {
		klog.Warningf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func (whs *WebhookServer) InjectProxy(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) InjectBroker(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) getProxySidecarConfig(proxyCM string, serverAddr string, serverPort int32) (sc *SidecarConfig, err error) {
	sc = whs.ProxySidecar.DeepCopy()
	sc.AddProxyIptablesRule(serverAddr, serverPort)
	sc.RenameVolumeRefCM(proxyCM, constants.DefaultProxyVolumeName)
	return
}

// 修改 iptables 的规则
func (whs *WebhookServer) getBrokerSidecarConfig(brokerCM string, serverContainerPort int32) (sc *SidecarConfig, err error) {
	sc = whs.BrokerSidecar.DeepCopy()
	sc.AddBrokerIptablesRule(serverContainerPort)
	sc.RenameVolumeRefCM(brokerCM, constants.DefaultBrokerVolumeName)
	return
}

func (whs *WebhookServer) parseModelServerAddr(ctx context.Context, deployName, namespace string) (address string, err error) {
	serverDeploy, err := whs.K8sClientset.AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
	if err != nil {
		return
	}

	nodeSelector := serverDeploy.Spec.Template.Spec.NodeSelector
	if len(nodeSelector) == 0 {
		err = fmt.Errorf("broker nodeSelector is empty in deploy [%s/%s]", namespace, deployName)
		return
	}

	nodeName, ok := nodeSelector[corev1.LabelHostname]
	if !ok {
		err = fmt.Errorf("broker node not specified in deploy [%s/%s]", namespace, deployName)
		return
	}

	nodeIP, err := whs.GetNodeAddress(ctx, nodeName)
	if err != nil {
		klog.Errorf("parse node [%s] internalIP failed", nodeName)
		return
	}

	address = nodeIP
	return
}

func (whs *WebhookServer) GetNodeAddress(ctx context.Context, nodeName string) (address string, err error) {
	node, err := whs.K8sClientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return
	}

	nodeLabels := node.GetLabels()
	if len(nodeLabels) > 0 {
		nodeAddrInKubeEdge, ok := nodeLabels["kubeedge.io/internal-ip"]
		if ok {
			address = nodeAddrInKubeEdge
			return
		}
	}

	if len(node.Status.Addresses) == 0 {
		err = fmt.Errorf("node [%s] addresses are empty", nodeName)
		return
	}

	found := false
	for _, nodeAddress := range node.Status.Addresses {
		if nodeAddress.Type == corev1.NodeInternalIP {
			found = true
			address = nodeAddress.Address
			return
		}
	}

	if !found {
		err = fmt.Errorf("node [%s] internal ip not found", nodeName)
	}

	return
}

func (whs *WebhookServer) GenerateProxyCM(pod *corev1.Pod) (proxyCM string, serverAddr string, serverPort int32, err error) {
	ctx := context.Background()
	namespace := pod.GetNamespace()
	podLabels := pod.GetLabels()
	podName := pod.GetName()

	imsName, ok := podLabels[constants.ServerDeployNameLabel]
	if !ok || imsName == "" {
		err = fmt.Errorf("server deploy name is missing in Pod [%s] labels", podName)
		return
	}

	imsNs, ok := podLabels[constants.ServerDeployNamespaceLabel]
	if !ok || imsNs == "" {
		err = fmt.Errorf("server deploy namespace is missing in Pod [%s] labels", podName)
		return
	}

	inferModelService, err := whs.CrdClientSet.AppsV1alpha1().InferModelServices(imsNs).Get(ctx, imsName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get inferModelService [%s/%s] failed", imsNs, imsName)
		return
	}

	appServiceGroup, ok := podLabels[constants.ProxyServiceGroupLabel]
	if !ok || appServiceGroup == "" {
		appServiceGroup = constants.DefaultServiceGroup
	}

	serverAddr, err = whs.parseModelServerAddr(ctx, imsNs, imsName)
	if err != nil {
		klog.Errorf("parse infer model server [%s/%s] addr failed", imsNs, imsName)
		return
	}

	serverPort = inferModelService.Spec.HostPort
	proxyConfig := inferModelService.Spec.ProxyConfig
	proxyConfig.Dispatch.Client = &proxyCfg.GRPCClient{
		Addr:    fmt.Sprintf("%s:%d", serverAddr, serverPort),
		Timeout: time.Second * 10,
	}

	proxyConfig.ServiceGroup = proxyCfg.ServiceGroup{
		Name: appServiceGroup,
	}

	ownerRef := metav1.GetControllerOf(pod)
	if ownerRef == nil {
		ownerRef = &metav1.OwnerReference{
			APIVersion: pod.APIVersion,
			Kind:       pod.Kind,
			Name:       podName,
			UID:        pod.UID,
		}
	}

	proxyCM = fmt.Sprintf("%s-proxy", ownerRef.Name)

	_, err = whs.K8sClientset.CoreV1().
		ConfigMaps(namespace).
		Get(ctx, proxyCM, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		var newContentBytes []byte
		newContentBytes, err = yaml.Marshal(inferModelService.Spec.ProxyConfig)
		if err != nil {
			return
		}

		targetConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            proxyCM,
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{*ownerRef},
			},
			Data: map[string]string{
				constants.EdgeQosProxyConfigName: string(newContentBytes),
			},
		}

		_, err = whs.K8sClientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), targetConfigMap, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = nil
			} else {
				klog.Errorf("create configmap [%s] to ns [%s] failed", proxyCM, namespace)
			}
		}
	}
	return
}

func (whs *WebhookServer) GenerateBrokerCM(pod *corev1.Pod) (brokerCM string, containerPort int32, err error) {
	ctx := context.Background()
	namespace := pod.GetNamespace()
	ownerRef := metav1.GetControllerOf(pod)
	if ownerRef == nil {
		err = fmt.Errorf("pod owner reference is nil")
		return
	}

	podOwnerName := ownerRef.Name
	inferModelService, err := whs.CrdClientSet.AppsV1alpha1().InferModelServices(namespace).Get(ctx, podOwnerName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get inferModelService [%s] failed", podOwnerName)
		return
	}

	containerPort = utils.GetTargetContainerPort(pod, inferModelService.Spec.HostPort)
	if containerPort == 0 {
		err = fmt.Errorf("container port is empty in pod [%s]", pod.GetName())
		return
	}

	brokerConfig := inferModelService.Spec.BrokerConfig
	brokerConfig.Dispatch.Client = &brokerCfg.GRPCClient{
		Addr:    fmt.Sprintf("127.0.0.1:%d", containerPort),
		Timeout: time.Second * 10,
	}

	// use deployment name as configMap name
	brokerCM = fmt.Sprintf("%s-broker", podOwnerName)
	_, err = whs.K8sClientset.CoreV1().
		ConfigMaps(namespace).
		Get(ctx, brokerCM, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		var newContentBytes []byte
		newContentBytes, err = yaml.Marshal(inferModelService.Spec.BrokerConfig)
		if err != nil {
			return
		}

		targetConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerCM,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       podOwnerName,
						UID:        ownerRef.UID,
					},
				},
			},
			Data: map[string]string{
				constants.EdgeQosBrokerConfigName: string(newContentBytes),
			},
		}

		_, err = whs.K8sClientset.CoreV1().ConfigMaps(namespace).Create(ctx, targetConfigMap, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = nil
			} else {
				klog.Errorf("create configmap [%s] to ns [%s] failed", brokerCM, namespace)
			}
		}

	}

	return
}

func (whs *WebhookServer) createSidecarPatch(pod *corev1.Pod, anno map[string]string) ([]byte, error) {
	var patch []patchOperation

	var cfg *SidecarConfig
	cfg, err := whs.prepareSidecarConfiguration(pod)
	if err != nil {
		klog.Errorf("prepare sidecar patch failed, %v", err)
		return nil, err
	}

	patch = append(patch, addContainer(pod.Spec.InitContainers, cfg.InitContainers, "/spec/initContainers")...)
	patch = append(patch, addContainer(pod.Spec.Containers, cfg.Containers, "/spec/containers")...)
	patch = append(patch, addVolume(pod.Spec.Volumes, cfg.Volumes, "/spec/volumes")...)
	patch = append(patch, updateAnnotation(pod.Annotations, anno)...)

	return json.Marshal(patch)
}

func (whs *WebhookServer) prepareSidecarConfiguration(pod *corev1.Pod) (sc *SidecarConfig, err error) {
	labels := pod.GetLabels()
	enableAppInjection, hasAppLabel := labels[constants.WebhookProxyInjectLabel]
	enableModelInjection, hasModelLabel := labels[constants.WebhookBrokerInjectLabel]

	if hasAppLabel && hasModelLabel {
		err = fmt.Errorf("app and model injection labels cannot exist at the same time")
		return
	}

	if hasAppLabel && enableAppInjection == constants.WebhookInjectEnable {
		if whs.ProxySidecar == nil {
			err = fmt.Errorf("app sidecar configuration is empty")
			return
		}

		var proxyCM string
		var serverAddr string
		var serverPort int32
		proxyCM, serverAddr, serverPort, err = whs.GenerateProxyCM(pod)
		if err != nil {
			return
		}

		sc, err = whs.getProxySidecarConfig(proxyCM, serverAddr, serverPort)
		return
	}

	if hasModelLabel && enableModelInjection == constants.WebhookInjectEnable {
		if whs.BrokerSidecar == nil {
			err = fmt.Errorf("model server sidecar configuration is empty")
			return
		}

		var brokerCM string
		var containerPort int32
		brokerCM, containerPort, err = whs.GenerateBrokerCM(pod)
		if err != nil {
			return
		}

		sc, err = whs.getBrokerSidecarConfig(brokerCM, containerPort)
		return
	}

	err = fmt.Errorf("no sidecar config found according to injection labels, %v", labels)
	return
}

func (whs *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request

	fmt.Printf("admissionReview Rquest Pod: %s", string(req.Object.Raw))
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Warningf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	if !mutationRequired(ignoredNamespaces, &pod.ObjectMeta) {
		klog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := map[string]string{admissionWebhookAnnotationStatusKey: "injected"}
	patchBytes, err := whs.createSidecarPatch(&pod, annotations)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func addContainer(target, added []corev1.Container, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolume(target, added []corev1.Volume, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil {
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			op := "add"
			if target[key] != "" {
				op = "replace"
			}
			patch = append(patch, patchOperation{
				Op:    op,
				Path:  "/metadata/annotations/" + escapeJSONPointerValue(key),
				Value: value,
			})
		}
	}

	return patch
}

func escapeJSONPointerValue(in string) string {
	step := strings.Replace(in, "~", "~0", -1)
	return strings.Replace(step, "/", "~1", -1)
}

func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta) bool {
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			klog.Infof("Skip mutation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	status := annotations[admissionWebhookAnnotationStatusKey]

	// determine whether to perform mutation based on annotation for the target resource
	var required bool
	if strings.ToLower(status) == "injected" {
		required = false
	} else {
		switch strings.ToLower(annotations[admissionWebhookAnnotationInjectKey]) {
		default:
			required = true
		case "n", "not", "false", "off":
			required = false
		}
	}

	klog.Infof("Mutation policy for %s/%s: status: %q required:%v", metadata.Namespace, metadata.Name, status, required)
	return required
}

// CreateOrUpdateMutatingWebhookConfiguration create Webhook CR
// Client application Pod with label "client.modelmesh.edgewize.io/injection=enabled",
// InferModel Pod with label "server.modelmesh.edgewize.io/injection=enabled"
func CreateOrUpdateMutatingWebhookConfiguration(k8sClientset *k8s.Clientset, caPEM *bytes.Buffer, webhookService, webhookNamespace string, webhookPort int32) error {
	mutatingWebhookConfigV1Client := k8sClientset.AdmissionregistrationV1()

	klog.Infof("Creating or updating the mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
	fail := admissionregistrationv1.Fail
	sideEffect := admissionregistrationv1.SideEffectClassNone
	mutatingWebhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.WebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:                    "proxy.edge-qos.edgewize.io",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			SideEffects:             &sideEffect,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(), // self-generated CA for the webhook
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &WebhookProxyInjectPath,
					Port:      &webhookPort,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.WebhookProxyInjectLabel: constants.WebhookInjectEnable,
				},
			},
			FailurePolicy: &fail,
		}, {
			Name:                    "broker.edge-qos.edgewize.io",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			SideEffects:             &sideEffect,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(), // self-generated CA for the webhook
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &WebHookBrokerInjectPath,
					Port:      &webhookPort,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.WebhookBrokerInjectLabel: constants.WebhookInjectEnable,
				},
			},
			FailurePolicy: &fail,
		}},
	}

	foundWebhookConfig, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Get(context.TODO(), constants.WebhookConfigName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		if _, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Create(context.TODO(), mutatingWebhookConfig, metav1.CreateOptions{}); err != nil {
			klog.Warningf("Failed to create the mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
			return err
		}
		klog.Infof("Created mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
	} else if err != nil {
		klog.Infof("Failed to check the mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
		return err
	} else {
		// there is an existing mutatingWebhookConfiguration
		if CheckWebhookUpdated(foundWebhookConfig, mutatingWebhookConfig) {
			mutatingWebhookConfig.ObjectMeta.ResourceVersion = foundWebhookConfig.ObjectMeta.ResourceVersion
			if _, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Update(context.TODO(), mutatingWebhookConfig, metav1.UpdateOptions{}); err != nil {
				klog.Warningf("Failed to update the mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
				return err
			}
			klog.Infof("Updated the mutatingwebhookconfiguration: %s", constants.WebhookConfigName)
		}
		klog.Infof("The mutatingwebhookconfiguration: %s already exists and has no change", constants.WebhookConfigName)
	}

	return nil
}

func CheckWebhookUpdated(oldCfg, newCfg *admissionregistrationv1.MutatingWebhookConfiguration) bool {
	if len(oldCfg.Webhooks) != len(newCfg.Webhooks) {
		return true
	}

	for index := range oldCfg.Webhooks {
		oldWebhook := oldCfg.Webhooks[index]
		newWebhook := newCfg.Webhooks[index]
		if !(oldWebhook.Name == newWebhook.Name &&
			reflect.DeepEqual(oldWebhook.AdmissionReviewVersions, newWebhook.AdmissionReviewVersions) &&
			reflect.DeepEqual(oldWebhook.SideEffects, newWebhook.SideEffects) &&
			reflect.DeepEqual(oldWebhook.FailurePolicy, newWebhook.FailurePolicy) &&
			reflect.DeepEqual(oldWebhook.Rules, newWebhook.Rules) &&
			reflect.DeepEqual(oldWebhook.NamespaceSelector, newWebhook.NamespaceSelector) &&
			reflect.DeepEqual(oldWebhook.ClientConfig.CABundle, newWebhook.ClientConfig.CABundle) &&
			reflect.DeepEqual(oldWebhook.ClientConfig.Service, newWebhook.ClientConfig.Service)) {
			return true
		}
	}
	return false
}
