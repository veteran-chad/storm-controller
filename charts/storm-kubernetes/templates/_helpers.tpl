{{/*
Helm helpers for storm-kubernetes chart
Uses Bitnami common library for most functionality
*/}}

{{/*
Validate Zookeeper configuration
*/}}
{{- define "storm.validateZookeeper" -}}
{{- if and (not .Values.zookeeper.enabled) (empty .Values.zookeeper.external.servers) }}
{{- fail "Zookeeper configuration error: Either zookeeper.enabled must be true or zookeeper.external.servers must be provided" }}
{{- end }}
{{- end -}}

{{/*
Get the Zookeeper headless service name
*/}}
{{- define "storm.zookeeperHeadlessService" -}}
{{- if .Values.zookeeper.fullnameOverride -}}
{{- printf "%s-headless" .Values.zookeeper.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-zookeeper-headless" .Release.Name -}}
{{- end -}}
{{- end -}}

{{/*
Get the Nimbus seeds (headless service name)
Since all resources are in the same namespace, we can use the simple service name
*/}}
{{- define "storm.nimbusSeed" -}}
{{- if .Values.nimbus.fullnameOverride -}}
{{- printf "%s-headless" .Values.nimbus.fullnameOverride -}}
{{- else -}}
{{- printf "%s-nimbus-headless" (include "common.names.fullname" .) -}}
{{- end -}}
{{- end -}}


{{/*
Create the name of the service account to use
*/}}
{{- define "storm.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "common.names.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Datadog log collection annotations
*/}}
{{- define "storm.datadogLogAnnotations" -}}
{{- $root := .root -}}
{{- $component := .component -}}
{{- if and $root.Values.metrics.datadog.enabled $root.Values.metrics.datadog.scrapeLogs -}}
ad.datadoghq.com/{{ $component }}.logs: '[{"source": "storm", "service": "{{ $root.Values.metrics.serviceName }}-{{ $component }}"}]'
{{- end -}}
{{- end -}}

{{/*
Datadog pod labels
*/}}
{{- define "storm.datadogLabels" -}}
{{- if .Values.metrics.datadog.enabled -}}
tags.datadoghq.com/env: {{ .Values.metrics.environment | quote }}
tags.datadoghq.com/service: {{ .Values.metrics.serviceName | quote }}
tags.datadoghq.com/version: {{ .Values.metrics.serviceVersion | quote }}
{{- end -}}
{{- end -}}



{{/*
Validate supervisor memory configuration
*/}}
{{- define "storm.supervisor.validateMemory" -}}
{{- if eq .Values.supervisor.memoryConfig.mode "auto" -}}
{{- if not .Values.supervisor.memoryConfig.memoryPerWorker -}}
{{- fail "supervisor.memoryConfig.memoryPerWorker is required when mode is 'auto'" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Calculate supervisor memory settings for container resources
*/}}
{{- define "storm.supervisor.memorySettings" -}}
{{- $memoryPerWorker := .Values.supervisor.memoryConfig.memoryPerWorker | toString -}}
{{- $slots := .Values.supervisor.slotsPerSupervisor -}}
{{- $overheadPercent := .Values.supervisor.memoryConfig.memoryOverheadPercent -}}
{{- $cpuPerWorker := .Values.supervisor.memoryConfig.cpuPerWorker | toString -}}
{{- $workerMemoryMB := 0 -}}
{{- if hasSuffix "Gi" $memoryPerWorker -}}
{{- $workerMemoryMB = trimSuffix "Gi" $memoryPerWorker | float64 | mul 1024 | int -}}
{{- else if hasSuffix "Mi" $memoryPerWorker -}}
{{- $workerMemoryMB = trimSuffix "Mi" $memoryPerWorker | int -}}
{{- end -}}
{{- $totalWorkerMemoryMB := mul $workerMemoryMB $slots -}}
{{- $containerMemoryMB := div (mul $totalWorkerMemoryMB (add 100 $overheadPercent)) 100 -}}
{{- $cpuCores := 1.0 -}}
{{- if hasSuffix "m" $cpuPerWorker -}}
{{- $cpuCores = trimSuffix "m" $cpuPerWorker | float64 | div 1000 -}}
{{- else -}}
{{- $cpuCores = $cpuPerWorker | float64 -}}
{{- end -}}
{{- $totalCpu := mulf $cpuCores (float64 $slots) -}}
containerMemory: {{ printf "%dMi" $containerMemoryMB }}
containerCpu: {{ printf "%.2f" $totalCpu }}
{{- end -}}