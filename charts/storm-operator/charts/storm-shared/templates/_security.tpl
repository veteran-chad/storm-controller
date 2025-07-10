{{/*
Pod Security Context
*/}}
{{- define "storm-shared.podSecurityContext" -}}
{{- $context := .podSecurityContext | default dict -}}
{{- if $context.enabled -}}
securityContext:
  {{- if $context.fsGroup }}
  fsGroup: {{ $context.fsGroup }}
  {{- end }}
  {{- if $context.fsGroupChangePolicy }}
  fsGroupChangePolicy: {{ $context.fsGroupChangePolicy }}
  {{- end }}
  {{- if $context.supplementalGroups }}
  supplementalGroups: {{- toYaml $context.supplementalGroups | nindent 4 }}
  {{- end }}
  {{- if $context.sysctls }}
  sysctls: {{- toYaml $context.sysctls | nindent 4 }}
  {{- end }}
{{- end -}}
{{- end -}}

{{/*
Container Security Context
*/}}
{{- define "storm-shared.containerSecurityContext" -}}
{{- $context := .containerSecurityContext | default dict -}}
{{- if $context.enabled -}}
securityContext:
  {{- if not (empty $context.runAsUser) }}
  runAsUser: {{ $context.runAsUser }}
  {{- end }}
  {{- if not (empty $context.runAsGroup) }}
  runAsGroup: {{ $context.runAsGroup }}
  {{- end }}
  {{- if not (empty $context.runAsNonRoot) }}
  runAsNonRoot: {{ $context.runAsNonRoot }}
  {{- end }}
  {{- if not (empty $context.privileged) }}
  privileged: {{ $context.privileged }}
  {{- end }}
  {{- if not (empty $context.readOnlyRootFilesystem) }}
  readOnlyRootFilesystem: {{ $context.readOnlyRootFilesystem }}
  {{- end }}
  {{- if not (empty $context.allowPrivilegeEscalation) }}
  allowPrivilegeEscalation: {{ $context.allowPrivilegeEscalation }}
  {{- end }}
  {{- if $context.capabilities }}
  capabilities:
    {{- if $context.capabilities.drop }}
    drop: {{- toYaml $context.capabilities.drop | nindent 4 }}
    {{- end }}
    {{- if $context.capabilities.add }}
    add: {{- toYaml $context.capabilities.add | nindent 4 }}
    {{- end }}
  {{- end }}
  {{- if $context.seLinuxOptions }}
  seLinuxOptions: {{- toYaml $context.seLinuxOptions | nindent 4 }}
  {{- end }}
  {{- if $context.seccompProfile }}
  seccompProfile: {{- toYaml $context.seccompProfile | nindent 4 }}
  {{- end }}
{{- end -}}
{{- end -}}