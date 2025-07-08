{{/*
Expand the name of the chart.
*/}}
{{- define "storm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "storm.fullname" -}}
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
{{- define "storm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "storm.labels" -}}
helm.sh/chart: {{ include "storm.chart" . }}
{{ include "storm.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.commonLabels }}
{{ toYaml .Values.commonLabels }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "storm.selectorLabels" -}}
app.kubernetes.io/name: {{ include "storm.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "storm.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "storm.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the proper Storm image name
*/}}
{{- define "storm.image" -}}
{{- include "common.images.image" (dict "imageRoot" .Values.image "global" .Values.global) -}}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "storm.imagePullSecrets" -}}
{{- include "common.images.pullSecrets" (dict "images" (list .Values.image) "global" .Values.global) -}}
{{- end -}}

{{/*
Create a default fully qualified nimbus name.
*/}}
{{- define "storm.nimbus.fullname" -}}
{{- printf "%s-%s" (include "storm.fullname" .) "nimbus" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified supervisor name.
*/}}
{{- define "storm.supervisor.fullname" -}}
{{- printf "%s-%s" (include "storm.fullname" .) "supervisor" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified ui name.
*/}}
{{- define "storm.ui.fullname" -}}
{{- printf "%s-%s" (include "storm.fullname" .) "ui" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Return the Zookeeper connection string
*/}}
{{- define "storm.zookeeper.connect" -}}
{{- if .Values.zookeeper.enabled }}
{{- printf "%s-zookeeper" .Release.Name }}
{{- else }}
{{- join "," .Values.externalZookeeper.servers }}
{{- end }}
{{- end }}

{{/*
Return the Storm configuration configmap name
*/}}
{{- define "storm.configmapName" -}}
{{- printf "%s-config" (include "storm.fullname" .) }}
{{- end }}

{{/*
Compile all warnings into a single message.
*/}}
{{- define "storm.validateValues" -}}
{{- $messages := list -}}
{{- $messages := append $messages (include "storm.validateValues.zookeeper" .) -}}
{{- $messages := append $messages (include "storm.validateValues.auth" .) -}}
{{- $messages := without $messages "" -}}
{{- $message := join "\n" $messages -}}

{{- if $message -}}
{{-   printf "\nVALUES VALIDATION:\n%s" $message -}}
{{- end -}}
{{- end -}}

{{/*
Validate values of Storm - Zookeeper
*/}}
{{- define "storm.validateValues.zookeeper" -}}
{{- if and (not .Values.zookeeper.enabled) (empty .Values.externalZookeeper.servers) -}}
storm: Zookeeper
    Either internal Zookeeper must be enabled or external Zookeeper servers must be provided.
    Please set either `zookeeper.enabled=true` or provide `externalZookeeper.servers`.
{{- end -}}
{{- end -}}

{{/*
Validate values of Storm - Authentication
*/}}
{{- define "storm.validateValues.auth" -}}
{{- if and .Values.ui.auth (and .Values.ui.auth.enabled (and (eq .Values.ui.auth.type "simple") (and (empty .Values.ui.auth.users) (empty .Values.ui.auth.existingSecret)))) -}}
storm: UI Authentication
    When simple authentication is enabled, you must provide either users or an existing secret.
    Please set either `ui.auth.users` or `ui.auth.existingSecret`.
{{- end -}}
{{- end -}}

{{/*
Return true if a secret object should be created for UI authentication
*/}}
{{- define "storm.ui.auth.createSecret" -}}
{{- if and .Values.ui.auth.enabled (empty .Values.ui.auth.existingSecret) -}}
    {{- true -}}
{{- end -}}
{{- end -}}

{{/*
Return the UI authentication secret name
*/}}
{{- define "storm.ui.auth.secretName" -}}
{{- if .Values.ui.auth.existingSecret -}}
    {{- .Values.ui.auth.existingSecret -}}
{{- else -}}
    {{- printf "%s-auth" (include "storm.ui.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Common Storm environment variables
*/}}
{{- define "storm.commonEnv" -}}
- name: STORM_ZOOKEEPER_SERVERS
  value: {{ include "storm.zookeeper.connect" . | quote }}
- name: STORM_NIMBUS_SEEDS
  value: {{ include "storm.nimbus.fullname" . | quote }}
- name: STORM_LOG_DIR
  value: "/logs"
- name: STORM_CLUSTER_MODE
  value: "distributed"
{{- end -}}

{{/*
Return pod anti-affinity preset
*/}}
{{- define "storm.podAntiAffinityPreset" -}}
{{- $component := .component -}}
{{- $context := .context -}}
{{- $preset := .preset -}}
{{- if eq $preset "soft" -}}
preferredDuringSchedulingIgnoredDuringExecution:
- weight: 100
  podAffinityTerm:
    labelSelector:
      matchLabels:
        {{- include "storm.selectorLabels" $context | nindent 8 }}
        app.kubernetes.io/component: {{ $component }}
    topologyKey: kubernetes.io/hostname
{{- else if eq $preset "hard" -}}
requiredDuringSchedulingIgnoredDuringExecution:
- labelSelector:
    matchLabels:
      {{- include "storm.selectorLabels" $context | nindent 6 }}
      app.kubernetes.io/component: {{ $component }}
  topologyKey: kubernetes.io/hostname
{{- end -}}
{{- end -}}

{{/*
Return node affinity preset
*/}}
{{- define "storm.nodeAffinityPreset" -}}
{{- $type := .type -}}
{{- $key := .key -}}
{{- $values := .values -}}
{{- if $values -}}
{{- if eq $type "soft" -}}
preferredDuringSchedulingIgnoredDuringExecution:
- weight: 100
  preference:
    matchExpressions:
    - key: {{ $key }}
      operator: In
      values:
      {{- range $values }}
      - {{ . | quote }}
      {{- end }}
{{- else if eq $type "hard" -}}
requiredDuringSchedulingIgnoredDuringExecution:
  nodeSelectorTerms:
  - matchExpressions:
    - key: {{ $key }}
      operator: In
      values:
      {{- range $values }}
      - {{ . | quote }}
      {{- end }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the nimbus headless service
*/}}
{{- define "storm.nimbus.headless.serviceName" -}}
{{- printf "%s-headless" (include "storm.nimbus.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create the name of the supervisor service
*/}}
{{- define "storm.supervisor.serviceName" -}}
{{- include "storm.supervisor.fullname" . }}
{{- end }}

{{/*
Create the name of the ui service
*/}}
{{- define "storm.ui.serviceName" -}}
{{- include "storm.ui.fullname" . }}
{{- end }}

{{/*
Create container ports list for Storm component
*/}}
{{- define "storm.containerPorts" -}}
{{- $component := . -}}
{{- if eq $component "nimbus" -}}
- name: thrift
  containerPort: 6627
  protocol: TCP
{{- else if eq $component "supervisor" -}}
- name: http
  containerPort: 8000
  protocol: TCP
{{- else if eq $component "ui" -}}
- name: http
  containerPort: 8080
  protocol: TCP
{{- else if eq $component "controller" -}}
- name: http
  containerPort: 8080
  protocol: TCP
- name: metrics
  containerPort: 8081
  protocol: TCP
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for HPA
*/}}
{{- define "storm.hpa.apiVersion" -}}
{{- if semverCompare ">=1.23-0" .Capabilities.KubeVersion.GitVersion -}}
{{- print "autoscaling/v2" -}}
{{- else -}}
{{- print "autoscaling/v2beta2" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for PodDisruptionBudget
*/}}
{{- define "storm.pdb.apiVersion" -}}
{{- if semverCompare ">=1.21-0" .Capabilities.KubeVersion.GitVersion -}}
{{- print "policy/v1" -}}
{{- else -}}
{{- print "policy/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for Ingress
*/}}
{{- define "storm.ingress.apiVersion" -}}
{{- if and .Values.ui.ingress.enabled (not .Values.ui.ingress.apiVersion) -}}
{{- if semverCompare ">=1.19-0" .Capabilities.KubeVersion.GitVersion -}}
{{- print "networking.k8s.io/v1" -}}
{{- else if semverCompare ">=1.14-0" .Capabilities.KubeVersion.GitVersion -}}
{{- print "networking.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "extensions/v1beta1" -}}
{{- end -}}
{{- else if .Values.ui.ingress.apiVersion -}}
{{- .Values.ui.ingress.apiVersion -}}
{{- end -}}
{{- end -}}

{{/*
Common annotations
*/}}
{{- define "storm.commonAnnotations" -}}
{{- if .Values.commonAnnotations }}
{{ toYaml .Values.commonAnnotations }}
{{- end }}
{{- end -}}

{{/*
Check if there are rolling tags in the images
*/}}
{{- define "storm.ui.ingress.certManagerRequest" -}}
{{ if or (hasKey . "cert-manager.io/cluster-issuer") (hasKey . "cert-manager.io/issuer") }}
    {{- true -}}
{{- end -}}
{{- end -}}