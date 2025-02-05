package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	"github.com/edgewize/edgeQ/pkg/client/clientset/versioned"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/utils"
	"gopkg.in/yaml.v2"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"net/http"
	"reflect"
	"regexp"
	"strings"
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
	cronJobNameRegexp = regexp.MustCompile(`(.+)-\d{8,10}$`)
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

func NewWebHookServer(name string, namespace string, port int32,
	proxyConf *SidecarConfig, brokerConf *SidecarConfig, kubeconfig string) (ws *WebhookServer, err error) {
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

func (sc *SidecarConfig) Clone() *SidecarConfig {
	newSidecarConfig := &SidecarConfig{
		InitContainers: []corev1.Container{},
		Containers:     []corev1.Container{},
		Volumes:        []corev1.Volume{},
	}

	for _, initContainer := range sc.InitContainers {
		newSidecarConfig.InitContainers = append(newSidecarConfig.InitContainers, initContainer)
	}

	for _, container := range sc.Containers {
		newSidecarConfig.Containers = append(newSidecarConfig.Containers, container)
	}

	for _, volume := range sc.Volumes {
		newSidecarConfig.Volumes = append(newSidecarConfig.Volumes, volume)
	}

	return newSidecarConfig
}

func (sc *SidecarConfig) AddProxyIptablesRule(serverAddr string, serverPort int32) {
	if len(sc.InitContainers) == 0 {
		return
	}

	initContainer := sc.InitContainers[0]
	initContainer.Command = []string{"/bin/sh", "-c"}
	initContainer.Args = []string{
		fmt.Sprintf("iptables -t nat -A OUTPUT -p tcp -d %s --dport %d -m owner ! --uid-owner %s -j DNAT --to-destination 127.0.0.1:%s",
			serverAddr, serverPort, constants.DefaultUserID, constants.DefaultProxyContainerPort),
	}

	sc.InitContainers = []corev1.Container{initContainer}
}

func (sc *SidecarConfig) AddTargetToEnv(targetPort int32, targetAddr string) {
	newContainers := []corev1.Container{}
	for _, container := range sc.Containers {
		container.Env = append(container.Env, corev1.EnvVar{Name: "TARGET_PORT", Value: fmt.Sprintf("%d", targetPort)})
		if targetAddr != "" {
			container.Env = append(container.Env, corev1.EnvVar{Name: "TARGET_ADDRESS", Value: targetAddr})
		}
		newContainers = append(newContainers, container)
	}

	sc.Containers = newContainers
}

func (sc *SidecarConfig) AddBrokerIptablesRule(serverContainerPort int32) {
	if len(sc.InitContainers) == 0 {
		return
	}

	initContainer := sc.InitContainers[0]
	initContainer.Command = []string{"/bin/sh", "-c"}
	initContainer.Args = []string{
		fmt.Sprintf("iptables -t nat -A PREROUTING -p tcp  --dport %d -j REDIRECT --to-port %s;",
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

	if _, err := w.Write(resp); err != nil {
		klog.Warningf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

	klog.Info("webhook injection request handled")
}

func (whs *WebhookServer) InjectProxy(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) InjectBroker(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) getProxySidecarConfig(proxyCM string, serverAddr string, serverPort int32) (sc *SidecarConfig, err error) {
	sc = whs.ProxySidecar.Clone()
	sc.AddProxyIptablesRule(serverAddr, serverPort)
	sc.AddTargetToEnv(serverPort, serverAddr)
	sc.RenameVolumeRefCM(proxyCM, constants.DefaultProxyVolumeName)
	return
}

// 修改 iptables 的规则
func (whs *WebhookServer) getBrokerSidecarConfig(brokerCM string, serverContainerPort int32) (sc *SidecarConfig, err error) {
	sc = whs.BrokerSidecar.Clone()
	sc.AddBrokerIptablesRule(serverContainerPort)
	sc.AddTargetToEnv(serverContainerPort, "")
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

	for _, nodeAddress := range node.Status.Addresses {
		if nodeAddress.Type == corev1.NodeInternalIP {
			address = nodeAddress.Address
			return
		}
	}

	err = fmt.Errorf("node [%s] internal ip not found", nodeName)
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

	serverAddr, err = whs.parseModelServerAddr(ctx, imsName, imsNs)
	if err != nil {
		klog.Errorf("parse infer model server [%s/%s] addr failed", imsNs, imsName)
		return
	}

	serverPort = inferModelService.Spec.HostPort
	proxyConfig := inferModelService.Spec.ProxyConfig

	proxyConfig.ServiceGroup = proxyCfg.ServiceGroup{
		Name: appServiceGroup,
	}

	proxyCM = fmt.Sprintf("%s-proxy", imsName)
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
				Name:      proxyCM,
				Namespace: namespace,
			},
			Data: map[string]string{
				constants.EdgeQosProxyConfigName: string(newContentBytes),
			},
		}

		ownerRef := metav1.GetControllerOf(pod)
		if ownerRef != nil {
			targetConfigMap.OwnerReferences = []metav1.OwnerReference{*ownerRef}
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

	deployMeta, _ := GetDeployMetaFromPod(pod)
	if deployMeta.Name == "" {
		err = fmt.Errorf("deployment name is empty")
		return
	}

	inferModelService, err := whs.CrdClientSet.AppsV1alpha1().InferModelServices(deployMeta.Namespace).Get(ctx, deployMeta.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get infer model service [%s] failed", deployMeta.Name)
		return
	}

	containerPort = utils.GetTargetContainerPort(pod, inferModelService.Spec.HostPort)
	if containerPort == 0 {
		err = fmt.Errorf("container port is empty in pod [%s]", pod.GetName())
		return
	}

	// use deployment name as configMap name
	brokerCM = fmt.Sprintf("%s-broker", deployMeta.Name)
	_, err = whs.K8sClientset.CoreV1().
		ConfigMaps(deployMeta.Namespace).
		Get(ctx, brokerCM, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		var k8sDeploy *v1.Deployment
		k8sDeploy, err = whs.K8sClientset.AppsV1().Deployments(deployMeta.Namespace).Get(ctx, deployMeta.Name, metav1.GetOptions{})
		if err != nil {
			return
		}

		var newContentBytes []byte
		newContentBytes, err = yaml.Marshal(inferModelService.Spec.BrokerConfig)
		if err != nil {
			return
		}

		targetConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerCM,
				Namespace: k8sDeploy.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       k8sDeploy.GetName(),
						UID:        k8sDeploy.GetUID(),
					},
				},
			},
			Data: map[string]string{
				constants.EdgeQosBrokerConfigName: string(newContentBytes),
			},
		}

		_, err = whs.K8sClientset.CoreV1().ConfigMaps(k8sDeploy.GetNamespace()).Create(ctx, targetConfigMap, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = nil
			} else {
				klog.Errorf("create configmap [%s] to ns [%s] failed", brokerCM, k8sDeploy.GetNamespace())
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
	enableAppInjection, hasProxyLabel := labels[constants.WebhookProxyInjectLabel]
	enableModelInjection, hasBrokerLabel := labels[constants.WebhookBrokerInjectLabel]

	if hasProxyLabel && hasBrokerLabel {
		err = fmt.Errorf("app and model injection labels cannot exist at the same time")
		return
	}

	if hasProxyLabel && enableAppInjection == constants.WebhookInjectEnable {
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

	if hasBrokerLabel && enableModelInjection == constants.WebhookInjectEnable {
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

	if pod.ObjectMeta.Namespace == "" {
		pod.ObjectMeta.Namespace = req.Namespace
	}

	if !whs.mutationRequired(ignoredNamespaces, &pod) {
		klog.Infof("Skipping mutation for [%s/%s] due to policy check", pod.Namespace, pod.Name)
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

func (whs *WebhookServer) mutationRequired(ignoredList []string, pod *corev1.Pod) bool {
	for _, namespace := range ignoredList {
		if pod.GetNamespace() == namespace {
			klog.Infof("Skip mutation for [%s] for it's in special namespace:%s", pod.GetName(), pod.GetNamespace())
			return false
		}
	}

	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	status := annotations[admissionWebhookAnnotationStatusKey]
	if strings.ToLower(status) == "injected" {
		return false
	} else {
		switch strings.ToLower(annotations[admissionWebhookAnnotationInjectKey]) {
		case "n", "not", "false", "off":
			return false
		default:
		}
	}

	podLabels := pod.GetLabels()
	if len(podLabels) == 0 {
		return false
	}

	_, isBroker := podLabels[constants.WebhookBrokerInjectLabel]
	if isBroker {
		deployMeta, _ := GetDeployMetaFromPod(pod)
		if deployMeta.Name == "" {
			return false
		}

		_, err := whs.CrdClientSet.
			AppsV1alpha1().
			InferModelServices(deployMeta.Namespace).
			Get(context.Background(), deployMeta.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(3).Infof("infer model service [%s] not found", deployMeta.Name)
				return false
			}

			klog.Errorf("get infer model service [%s] failed: %v", deployMeta.Name, err)
			return false
		}
	}

	return true
}

// GetDeployMetaFromPod heuristically derives deployment metadata from the pod spec.
func GetDeployMetaFromPod(pod *corev1.Pod) (types.NamespacedName, metav1.TypeMeta) {
	if pod == nil {
		return types.NamespacedName{}, metav1.TypeMeta{}
	}

	deployMeta := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}

	typeMetadata := metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}
	if len(pod.GenerateName) > 0 {
		// if the pod name was generated (or is scheduled for generation), we can begin an investigation into the controlling reference for the pod.
		var controllerRef metav1.OwnerReference
		controllerFound := false
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Controller != nil && *ref.Controller {
				controllerRef = ref
				controllerFound = true
				break
			}
		}
		if controllerFound {
			typeMetadata.APIVersion = controllerRef.APIVersion
			typeMetadata.Kind = controllerRef.Kind

			// heuristic for deployment detection
			deployMeta.Name = controllerRef.Name
			if typeMetadata.Kind == "ReplicaSet" && pod.Labels["pod-template-hash"] != "" && strings.HasSuffix(controllerRef.Name, pod.Labels["pod-template-hash"]) {
				name := strings.TrimSuffix(controllerRef.Name, "-"+pod.Labels["pod-template-hash"])
				deployMeta.Name = name
				typeMetadata.Kind = "Deployment"
			} else if typeMetadata.Kind == "ReplicaSet" && pod.Labels["rollouts-pod-template-hash"] != "" &&
				strings.HasSuffix(controllerRef.Name, pod.Labels["rollouts-pod-template-hash"]) {
				// Heuristic for ArgoCD Rollout
				name := strings.TrimSuffix(controllerRef.Name, "-"+pod.Labels["rollouts-pod-template-hash"])
				deployMeta.Name = name
				typeMetadata.Kind = "Rollout"
				typeMetadata.APIVersion = "v1alpha1"
			} else if typeMetadata.Kind == "ReplicationController" && pod.Labels["deploymentconfig"] != "" {
				// If the pod is controlled by the replication controller, which is created by the DeploymentConfig resource in
				// Openshift platform, set the deploy name to the deployment config's name, and the kind to 'DeploymentConfig'.
				//
				// nolint: lll
				// For DeploymentConfig details, refer to
				// https://docs.openshift.com/container-platform/4.1/applications/deployments/what-deployments-are.html#deployments-and-deploymentconfigs_what-deployments-are
				//
				// For the reference to the pod label 'deploymentconfig', refer to
				// https://github.com/openshift/library-go/blob/7a65fdb398e28782ee1650959a5e0419121e97ae/pkg/apps/appsutil/const.go#L25
				deployMeta.Name = pod.Labels["deploymentconfig"]
				typeMetadata.Kind = "DeploymentConfig"
			} else if typeMetadata.Kind == "Job" {
				// If job name suffixed with `-<digit-timestamp>`, where the length of digit timestamp is 8~10,
				// trim the suffix and set kind to cron job.
				if jn := cronJobNameRegexp.FindStringSubmatch(controllerRef.Name); len(jn) == 2 {
					deployMeta.Name = jn[1]
					typeMetadata.Kind = "CronJob"
					// heuristically set cron job api version to v1 as it cannot be derived from pod metadata.
					typeMetadata.APIVersion = "batch/v1"
				}
			}
		}
	}

	if deployMeta.Name == "" {
		// if we haven't been able to extract a deployment name, then just give it the pod name
		deployMeta.Name = pod.Name
	}

	return deployMeta, typeMetadata
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

func potentialPodName(metadata metav1.ObjectMeta) string {
	if metadata.Name != "" {
		return metadata.Name
	}
	if metadata.GenerateName != "" {
		return metadata.GenerateName + "***** (actual name not yet known)"
	}
	return ""
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

// CreateOrUpdateMutatingWebhookConfiguration create Webhook CR
// Client application Pod with label "proxy.edge-qos.edgewize.io/injection=enabled",
// InferModel Pod with label "broker.edge-qos.edgewize.io/injection=enabled"
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
