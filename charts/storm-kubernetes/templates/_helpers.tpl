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
Render cluster configuration with proper precedence
This helper filters out configuration keys that are managed by specific sections (zookeeper, ui, nimbus, supervisor, etc.)
to avoid duplication. Section-specific configs take precedence over clusterConfig.
*/}}
{{- define "storm.renderClusterConfig" -}}
{{- if .Values.clusterConfig }}
{{- range $key, $value := .Values.clusterConfig }}
{{- if and (ne $key "storm.zookeeper.servers") (ne $key "nimbus.seeds") }}
{{- if not (and $.Values.zookeeper.extraConfig (hasKey $.Values.zookeeper.extraConfig $key)) }}
{{- if not (and $.Values.ui.enabled $.Values.ui.extraConfig (hasKey $.Values.ui.extraConfig $key)) }}
{{- if not (and $.Values.nimbus.enabled $.Values.nimbus.extraConfig (hasKey $.Values.nimbus.extraConfig $key)) }}
{{- if not (and $.Values.supervisor.enabled $.Values.supervisor.extraConfig (hasKey $.Values.supervisor.extraConfig $key)) }}
{{- if not (and $.Values.cluster.enabled $.Values.cluster.extraConfig (hasKey $.Values.cluster.extraConfig $key)) }}
{{ $key }}: {{ $value | toJson }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}