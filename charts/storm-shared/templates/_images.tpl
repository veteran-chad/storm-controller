{{/*
Return the proper image registry
*/}}
{{- define "storm-shared.images.registry" -}}
{{- $registryName := .imageRoot.registry -}}
{{- if .global }}
  {{- if .global.imageRegistry }}
    {{- $registryName = .global.imageRegistry -}}
  {{- end -}}
{{- end -}}
{{- printf "%s" $registryName -}}
{{- end -}}

{{/*
Return the proper image repository
*/}}
{{- define "storm-shared.images.repository" -}}
{{- $repository := .imageRoot.repository -}}
{{- if .defaultRepository }}
  {{- if not $repository }}
    {{- $repository = .defaultRepository -}}
  {{- end -}}
{{- end -}}
{{- printf "%s" $repository -}}
{{- end -}}

{{/*
Return the proper image name
*/}}
{{- define "storm-shared.images.image" -}}
{{- $registryName := include "storm-shared.images.registry" . -}}
{{- $repositoryName := include "storm-shared.images.repository" . -}}
{{- $separator := ":" -}}
{{- $termination := .imageRoot.tag | toString -}}
{{- if .imageRoot.digest }}
  {{- $separator = "@" -}}
  {{- $termination = .imageRoot.digest | toString -}}
{{- end -}}
{{- if $registryName }}
  {{- printf "%s/%s%s%s" $registryName $repositoryName $separator $termination -}}
{{- else -}}
  {{- printf "%s%s%s" $repositoryName $separator $termination -}}
{{- end -}}
{{- end -}}

{{/*
Return the proper image pull policy
*/}}
{{- define "storm-shared.images.pullPolicy" -}}
{{- $pullPolicy := .imageRoot.pullPolicy -}}
{{- if .global }}
  {{- if .global.imagePullPolicy }}
    {{- $pullPolicy = .global.imagePullPolicy -}}
  {{- end -}}
{{- end -}}
{{- if not $pullPolicy }}
  {{- if contains "latest" (toString .imageRoot.tag) }}
    {{- $pullPolicy = "Always" -}}
  {{- else -}}
    {{- $pullPolicy = "IfNotPresent" -}}
  {{- end -}}
{{- end -}}
{{- printf "%s" $pullPolicy -}}
{{- end -}}

{{/*
Compile all warnings for images
*/}}
{{- define "storm-shared.images.validateValues" -}}
{{- $messages := list -}}
{{- if and .imageRoot.registry (not .imageRoot.repository) -}}
  {{- $messages = append $messages (printf "Image registry is specified but repository is not for %s" .component) -}}
{{- end -}}
{{- if and .imageRoot.digest .imageRoot.tag -}}
  {{- $messages = append $messages (printf "Both tag and digest are specified for %s image, digest will take precedence" .component) -}}
{{- end -}}
{{- include "storm-shared.messages.error" (dict "messages" $messages "context" $) -}}
{{- end -}}