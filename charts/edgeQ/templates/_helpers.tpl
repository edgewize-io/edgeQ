{{/*
Expand the name of the chart.
*/}}
{{- define "edge-qos-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "edge-qos-controller.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "edge-qos-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "edge-qos-controller.labels" -}}
helm.sh/chart: {{ include "edge-qos-controller.chart" . }}
{{ include "edge-qos-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "edge-qos-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "edge-qos-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "edge-qos-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "edge-qos-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "edge-qos-component.image" -}}
{{- $registryName := .global.imageRegistry -}}
{{- $name := .component.name -}}
{{- $separator := ":" -}}
{{- $termination := "latest" | toString -}}
{{- if and .component.imageRegistry (ne .component.imageRegistry "") }}
    {{- $registryName = .component.imageRegistry -}}
{{- end -}}
{{- if .component.tag }}
    {{- $termination = .component.tag | toString -}}
{{- end -}}
{{- if .component.digest }}
    {{- $separator = "@" -}}
    {{- $termination = .component.digest | toString -}}
{{- end -}}
{{- if and $registryName (ne $registryName "") }}
    {{- printf "%s/%s%s%s" $registryName $name $separator $termination -}}
{{- else -}}
    {{- printf "%s%s%s" $name $separator $termination -}}
{{- end -}}
{{- end -}}

{{- define "edge-qos-controller.image" -}}
{{ include "edge-qos-component.image" (dict "component" .Values.imageReference.edgeQosController "global" .Values) }}
{{- end -}}

{{- define "edge-qos-broker.image" -}}
{{ include "edge-qos-component.image" (dict "component" .Values.imageReference.edgeQosBroker "global" .Values) }}
{{- end -}}

{{- define "edge-qos-proxy.image" -}}
{{ include "edge-qos-component.image" (dict "component" .Values.imageReference.edgeQosProxy "global" .Values) }}
{{- end -}}

{{- define "init-iptables.image" -}}
{{ include "edge-qos-component.image" (dict "component" .Values.imageReference.initIptables "global" .Values) }}
{{- end -}}


{{- define "model-mesh-component.imagePullSecrets" -}}
  {{- $pullSecrets := list }}

  {{- if .global.imagePullSecrets }}
    {{- range .global.imagePullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- if .component.imagePullSecrets }}
    {{- range .component.imagePullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- if (not (empty $pullSecrets)) }}
imagePullSecrets:
    {{- range $item := $pullSecrets }}
  - name: {{ $item.name }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "edge-qos-controller.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.edgeQosController "global" .Values) -}}
{{- end -}}

{{- define "edge-qos-broker.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.edgeQosBroker "global" .Values) -}}
{{- end -}}

{{- define "edge-qos-proxy.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.edgeQosProxy "global" .Values) -}}
{{- end -}}
