package msc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	brokerCfg "github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/pkg/constants"
	"gopkg.in/yaml.v2"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/rand"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var (
	WebhookConfigName                   = "msc-sidecar-injector-webhook"
	WebhookApplicationInjectPath        = "/application"
	WebHookModelInjectPath              = "/model"
	WebhookApplicationInjectLabel       = "client.modelmesh.edgewize.io/injection"
	WebhookModelServerInjectLabel       = "server.modelmesh.edgewize.io/injection"
	WebhookInjectEnable                 = "enabled"
	admissionWebhookAnnotationInjectKey = "sidecar-injector-webhook.edgewize.io/inject"
	admissionWebhookAnnotationStatusKey = "sidecar-injector-webhook.edgewize.io/status"
	runtimeScheme                       = runtime.NewScheme()
	codecs                              = serializer.NewCodecFactory(runtimeScheme)
	deserializer                        = codecs.UniversalDeserializer()
	ignoredNamespaces                   = []string{
		metav1.NamespaceSystem,
		metav1.NamespacePublic,
	}
	AppProxyConfigName     = "model-mesh-proxy"
	ServerBrokerConfigName = "model-mesh-broker"
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
	Clientset        *k8s.Clientset
	CurrentNamespace string
}

type SidecarConfig struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

func (sc *SidecarConfig) DeepCopy() *SidecarConfig {
	newSidecarConfig := &SidecarConfig{
		Containers: []corev1.Container{},
		Volumes:    []corev1.Volume{},
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

func (sc *SidecarConfig) AddEnvironments(envs map[string]string) {
	containersWithEnv := []corev1.Container{}
	for _, container := range sc.Containers {
		for key, value := range envs {
			envItem := corev1.EnvVar{
				Name:  key,
				Value: value,
			}

			container.Env = append(container.Env, envItem)
		}

		containersWithEnv = append(containersWithEnv, container)
	}

	sc.Containers = containersWithEnv
}

func (sc *SidecarConfig) SetBrokerHostPort(containerName, portName string, hostPort int32) {
	newContainers := []corev1.Container{}
	for _, container := range sc.Containers {
		if container.Name == containerName {
			newPorts := []corev1.ContainerPort{}
			for _, portInfo := range container.Ports {
				if portInfo.Name == portName {
					portInfo.HostPort = hostPort
				}

				newPorts = append(newPorts, portInfo)
			}

			container.Ports = newPorts
		}

		newContainers = append(newContainers, container)
	}

	sc.Containers = newContainers
}

func (sc *SidecarConfig) RenameBrokerConfigMap(specifiedConfigName string) {
	newVolumes := []corev1.Volume{}
	for _, volume := range sc.Volumes {
		if volume.Name == constants.DefaultBrokerVolumeName {
			volume.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: specifiedConfigName,
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

func (whs *WebhookServer) InjectApplication(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) InjectModel(w http.ResponseWriter, r *http.Request) {
	whs.inject(w, r)
}

func (whs *WebhookServer) getProxySidecarConfig(podLabels map[string]string, podName string) (sc *SidecarConfig, err error) {
	sc = whs.ProxySidecar.DeepCopy()
	ctx := context.TODO()

	if len(podLabels) == 0 {
		err = fmt.Errorf("meta information is missing in Pod [%s] labels", podName)
		return
	}

	serverDeployName, ok := podLabels[constants.ServerDeployNameLabel]
	if !ok {
		err = fmt.Errorf("server deploy name is missing in Pod [%s] labels", podName)
		return
	}

	serverDeployNamespace, ok := podLabels[constants.ServerDeployNamespaceLabel]
	if !ok {
		err = fmt.Errorf("server deploy namespace is missing in Pod [%s] labels", podName)
		return
	}

	appServiceGroup, ok := podLabels[constants.ProxyServiceGroupLabel]
	if !ok || appServiceGroup == "" {
		appServiceGroup = constants.DefaultServiceGroup
	}

	brokerEndpoint, err := whs.parseTargetBrokerEndpoint(ctx, serverDeployName, serverDeployNamespace)
	if err != nil {
		klog.Errorf("fail to parse nodeIP:hostPort for deployment [%s/%s], %v", serverDeployNamespace, serverDeployName, err)
		return
	}

	proxyEnvs := map[string]string{
		constants.ProxyServiceGroupEnv:    appServiceGroup,
		constants.ServerBrokerEndpointEnv: brokerEndpoint,
	}

	sc.AddEnvironments(proxyEnvs)
	return
}

func (whs *WebhookServer) getBrokerSidecarConfig(specifiedConfigName string, podLabels map[string]string) (sc *SidecarConfig, err error) {
	sc = whs.BrokerSidecar.DeepCopy()

	brokerHostPortValue := GetHostPortFromLabel(podLabels)
	brokerHostPort, err := strconv.Atoi(brokerHostPortValue)
	if err != nil {
		return
	}

	sc.SetBrokerHostPort(constants.DefaultBrokerContainerName, constants.DefaultBrokerPortName, int32(brokerHostPort))
	sc.RenameBrokerConfigMap(specifiedConfigName)
	return
}

func (whs *WebhookServer) parseTargetBrokerEndpoint(ctx context.Context, deploymentName, namespace string) (endpoint string, err error) {
	serverDeploy, err := whs.Clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return
	}

	brokerHostPort := GetHostPortFromLabel(serverDeploy.Spec.Template.GetLabels())

	nodeSelector := serverDeploy.Spec.Template.Spec.NodeSelector
	if len(nodeSelector) == 0 {
		err = fmt.Errorf("broker nodeSelector is empty in deploy [%s/%s]", namespace, deploymentName)
		return
	}

	nodeName, ok := nodeSelector[corev1.LabelHostname]
	if !ok {
		err = fmt.Errorf("broker node not specified in deploy [%s/%s]", namespace, deploymentName)
		return
	}

	nodeIP, err := whs.GetNodeAddress(ctx, nodeName)
	if err != nil {
		klog.Errorf("parse node [%s] internalIP failed", nodeName)
		return
	}

	endpoint = fmt.Sprintf("%s:%s", nodeIP, brokerHostPort)
	return
}

func (whs *WebhookServer) GetNodeAddress(ctx context.Context, nodeName string) (address string, err error) {
	node, err := whs.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
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

func (whs *WebhookServer) copyProxyConfig(namespace string) (err error) {
	ctx := context.Background()
	originConfigMap, err := whs.Clientset.CoreV1().
		ConfigMaps(whs.CurrentNamespace).
		Get(ctx, AppProxyConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get origin configmap [%s] failed", AppProxyConfigName)
		return
	}

	targetConfigMap, err := whs.Clientset.CoreV1().
		ConfigMaps(namespace).
		Get(ctx, AppProxyConfigName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		targetConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AppProxyConfigName,
				Namespace: namespace,
			},
			Data: originConfigMap.Data,
		}

		_, err = whs.Clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), targetConfigMap, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = nil
			} else {
				klog.Errorf("create configmap [%s] to ns [%s] failed", AppProxyConfigName, namespace)
			}
		}
	}
	return
}

func (whs *WebhookServer) copyBrokerConfig(podLabels map[string]string, podName string) (specifiedConfigName string, err error) {
	ctx := context.TODO()
	if len(podLabels) == 0 {
		err = fmt.Errorf("meta information is missing in Pod [%s] labels", podName)
		return
	}

	serverDeployName, ok := podLabels[constants.ServerDeployNameLabel]
	if !ok {
		err = fmt.Errorf("server deploy name is missing in Pod [%s] labels", podName)
		return
	}

	serverDeployNamespace, ok := podLabels[constants.ServerDeployNamespaceLabel]
	if !ok {
		err = fmt.Errorf("server deploy namespace is missing in Pod [%s] labels", podName)
		return
	}

	serverDeploy, err := whs.Clientset.AppsV1().
		Deployments(serverDeployNamespace).
		Get(ctx, serverDeployName, metav1.GetOptions{})
	if err != nil {
		return
	}

	originConfigMap, err := whs.Clientset.CoreV1().
		ConfigMaps(whs.CurrentNamespace).
		Get(ctx, ServerBrokerConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get origin configmap [%s] failed", ServerBrokerConfigName)
		return
	}

	// use deployment name as configMap name
	specifiedConfigName = fmt.Sprintf("%s-%s", serverDeployName, rand.String(5))
	_, err = whs.Clientset.CoreV1().
		ConfigMaps(serverDeployNamespace).
		Get(ctx, specifiedConfigName, metav1.GetOptions{})
	if err == nil {
		return
	}

	if err != nil && apierrors.IsNotFound(err) {
		cfgDataMap := originConfigMap.Data
		if len(cfgDataMap) == 0 {
			err = fmt.Errorf("origin configMap %s data is empty", originConfigMap)
			return
		}

		brokerConfigFileContent, ok := cfgDataMap[constants.ServerBrokerConfigFileName]
		if !ok {
			err = fmt.Errorf("%s is empty in broker origin configmap", constants.ServerBrokerConfigFileName)
			return
		}

		newBrokerServerConfig := &brokerCfg.Config{}
		err = yaml.Unmarshal([]byte(brokerConfigFileContent), newBrokerServerConfig)
		if err != nil {
			return
		}

		var newBrokerServiceGroups []*brokerCfg.ServiceGroup
		newBrokerServiceGroups, err = GetServiceGroupFromLabel(podLabels)
		if err != nil {
			return
		}

		newBrokerServerConfig.ServiceGroups = newBrokerServiceGroups
		var newBrokerConfigFileContentBytes []byte
		newBrokerConfigFileContentBytes, err = yaml.Marshal(newBrokerServerConfig)
		if err != nil {
			return
		}

		targetConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      specifiedConfigName,
				Namespace: serverDeployNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       serverDeploy.Name,
						UID:        serverDeploy.UID,
					},
				},
			},
			Data: map[string]string{
				constants.ServerBrokerConfigFileName: string(newBrokerConfigFileContentBytes),
			},
		}

		_, err = whs.Clientset.CoreV1().ConfigMaps(serverDeployNamespace).Create(ctx, targetConfigMap, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = nil
			} else {
				klog.Errorf("create configmap [%s] to ns [%s] failed", specifiedConfigName, serverDeployNamespace)
			}
		}

	}

	return
}

func (whs *WebhookServer) createSidecarPatch(pod *corev1.Pod, anno map[string]string, namespace string) ([]byte, error) {
	var patch []patchOperation

	var cfg *SidecarConfig
	cfg, err := whs.prepareSidecarConfiguration(pod, namespace)
	if err != nil {
		klog.Errorf("prepare sidecar patch failed, %v", err)
		return nil, err
	}

	patch = append(patch, addContainer(pod.Spec.Containers, cfg.Containers, "/spec/containers")...)
	patch = append(patch, addVolume(pod.Spec.Volumes, cfg.Volumes, "/spec/volumes")...)
	patch = append(patch, updateAnnotation(pod.Annotations, anno)...)

	return json.Marshal(patch)
}

func (whs *WebhookServer) prepareSidecarConfiguration(pod *corev1.Pod, namespace string) (sc *SidecarConfig, err error) {
	labels := pod.GetLabels()
	enableAppInjection, hasAppLabel := labels[WebhookApplicationInjectLabel]
	enableModelInjection, hasModelLabel := labels[WebhookModelServerInjectLabel]

	if hasAppLabel && hasModelLabel {
		err = fmt.Errorf("app and model injection labels cannot exist at the same time")
		return
	}

	if hasAppLabel && enableAppInjection == WebhookInjectEnable {
		if whs.ProxySidecar == nil {
			err = fmt.Errorf("app sidecar configuration is empty")
			return
		}

		err = whs.copyProxyConfig(namespace)
		if err != nil {
			return
		}

		sc, err = whs.getProxySidecarConfig(labels, pod.GetName())
		return
	}

	if hasModelLabel && enableModelInjection == WebhookInjectEnable {
		if whs.BrokerSidecar == nil {
			err = fmt.Errorf("model server sidecar configuration is empty")
			return
		}

		var specifiedConfigName string
		specifiedConfigName, err = whs.copyBrokerConfig(labels, pod.GetName())
		if err != nil {
			return
		}

		sc, err = whs.getBrokerSidecarConfig(specifiedConfigName, labels)
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
	patchBytes, err := whs.createSidecarPatch(&pod, annotations, req.Namespace)
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
func CreateOrUpdateMutatingWebhookConfiguration(clientset *k8s.Clientset, caPEM *bytes.Buffer, webhookService, webhookNamespace string, webhookPort int32) error {
	mutatingWebhookConfigV1Client := clientset.AdmissionregistrationV1()

	klog.Infof("Creating or updating the mutatingwebhookconfiguration: %s", WebhookConfigName)
	fail := admissionregistrationv1.Fail
	sideEffect := admissionregistrationv1.SideEffectClassNone
	mutatingWebhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: WebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:                    "client.modelmesh.edgewize.io",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			SideEffects:             &sideEffect,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(), // self-generated CA for the webhook
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &WebhookApplicationInjectPath,
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
					WebhookApplicationInjectLabel: WebhookInjectEnable,
				},
			},
			FailurePolicy: &fail,
		}, {
			Name:                    "server.modelmesh.edgewize.io",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			SideEffects:             &sideEffect,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(), // self-generated CA for the webhook
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &WebHookModelInjectPath,
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
					WebhookModelServerInjectLabel: WebhookInjectEnable,
				},
			},
			FailurePolicy: &fail,
		}},
	}

	foundWebhookConfig, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Get(context.TODO(), WebhookConfigName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		if _, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Create(context.TODO(), mutatingWebhookConfig, metav1.CreateOptions{}); err != nil {
			klog.Warningf("Failed to create the mutatingwebhookconfiguration: %s", WebhookConfigName)
			return err
		}
		klog.Infof("Created mutatingwebhookconfiguration: %s", WebhookConfigName)
	} else if err != nil {
		klog.Infof("Failed to check the mutatingwebhookconfiguration: %s", WebhookConfigName)
		return err
	} else {
		// there is an existing mutatingWebhookConfiguration
		if CheckWebhookUpdated(foundWebhookConfig, mutatingWebhookConfig) {
			mutatingWebhookConfig.ObjectMeta.ResourceVersion = foundWebhookConfig.ObjectMeta.ResourceVersion
			if _, err := mutatingWebhookConfigV1Client.MutatingWebhookConfigurations().Update(context.TODO(), mutatingWebhookConfig, metav1.UpdateOptions{}); err != nil {
				klog.Warningf("Failed to update the mutatingwebhookconfiguration: %s", WebhookConfigName)
				return err
			}
			klog.Infof("Updated the mutatingwebhookconfiguration: %s", WebhookConfigName)
		}
		klog.Infof("The mutatingwebhookconfiguration: %s already exists and has no change", WebhookConfigName)
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

func GetHostPortFromLabel(labels map[string]string) (hostPort string) {
	hostPort = constants.DefaultBrokerHostPort
	if len(labels) == 0 {
		return
	}

	customHostPort, found := labels[constants.ServerBrokerHostPortLabel]
	if found {
		hostPort = customHostPort
	}

	return
}

func GetServiceGroupFromLabel(labels map[string]string) ([]*brokerCfg.ServiceGroup, error) {
	defaultWeight := int32(100)
	result := []*brokerCfg.ServiceGroup{}
	for key, value := range labels {
		if strings.HasPrefix(key, constants.ServiceGroupPrefix) {
			sgName := strings.TrimPrefix(key, constants.ServiceGroupPrefix)
			if sgName == "" {
				continue
			}

			sgWeight, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}

			result = append(result, &brokerCfg.ServiceGroup{
				Name:   sgName,
				Weight: int32(sgWeight),
			})

			defaultWeight -= int32(sgWeight)
		}
	}

	if defaultWeight < 0 {
		defaultWeight = int32(1)
		result = append(result, &brokerCfg.ServiceGroup{
			Name:   constants.DefaultServiceGroup,
			Weight: defaultWeight,
		})
	}

	return result, nil
}
