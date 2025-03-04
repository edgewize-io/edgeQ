package utils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func HasString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func CurrentNamespace() (string, error) {
	namespaceEnv := os.Getenv("NAMESPACE")
	if namespaceEnv != "" {
		return namespaceEnv, nil
	}

	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}

	return string(namespace), nil
}

func MergeLabels(additionLabels, originLabels map[string]string) map[string]string {
	if originLabels == nil {
		originLabels = make(map[string]string, len(additionLabels))
	}

	for key, value := range additionLabels {
		originLabels[key] = value
	}

	return originLabels
}

func GetTargetContainerPort(pod *corev1.Pod, hostPort int32) (containerPort int32) {
	for _, container := range pod.Spec.Containers {
		for _, portInfo := range container.Ports {
			if portInfo.HostPort == hostPort {
				containerPort = portInfo.ContainerPort
				return
			}
		}
	}

	return
}

func GetPodOwnerRefName(pod corev1.Pod) string {
	ownerRef := metav1.GetControllerOf(&pod)
	if ownerRef == nil {
		return pod.GetName()
	} else {
		return ownerRef.Name
	}
}
