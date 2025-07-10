{{/*
Storm memory configuration helpers

These templates calculate memory settings for Storm supervisors based on the
memoryConfig settings in values.yaml. This ensures consistent memory allocation
across container resources, JVM heap, and Storm's resource-aware scheduler.
*/}}

{{/*
Calculate supervisor memory settings based on configuration mode.
Returns a dict with:
  - containerMemory: Total memory for container (with overhead)
  - containerCpu: Total CPU for container
  - supervisorCapacityMB: Memory capacity to advertise to Nimbus
  - supervisorCpuCapacity: CPU capacity to advertise to Nimbus (x100)
  - workerHeapMB: Default heap size per worker
*/}}
{{- define "storm.supervisor.memorySettings" -}}
{{- if eq .Values.supervisor.memoryConfig.mode "auto" -}}
  {{- /* Auto mode: Calculate based on slots and per-worker memory */ -}}
  {{- $memPerWorkerMi := 0 -}}
  {{- if hasSuffix "Gi" .Values.supervisor.memoryConfig.memoryPerWorker -}}
    {{- $memPerWorkerMi = .Values.supervisor.memoryConfig.memoryPerWorker | trimSuffix "Gi" | float64 | mul 1024 | int -}}
  {{- else if hasSuffix "Mi" .Values.supervisor.memoryConfig.memoryPerWorker -}}
    {{- $memPerWorkerMi = .Values.supervisor.memoryConfig.memoryPerWorker | trimSuffix "Mi" | int -}}
  {{- else -}}
    {{- fail "memoryPerWorker must end with Gi or Mi" -}}
  {{- end -}}
  
  {{- $cpuPerWorker := .Values.supervisor.memoryConfig.cpuPerWorker | float64 -}}
  {{- $slots := .Values.supervisor.slotsPerSupervisor | int -}}
  {{- $overheadPercent := .Values.supervisor.memoryConfig.memoryOverheadPercent | float64 -}}
  
  {{- /* Calculate totals */ -}}
  {{- $totalWorkerMemoryMi := mul $memPerWorkerMi $slots -}}
  {{- $totalCpu := mul $cpuPerWorker $slots -}}
  {{- $overheadMultiplier := add 1.0 (div $overheadPercent 100.0) -}}
  {{- $containerMemoryMi := $totalWorkerMemoryMi | float64 | mul $overheadMultiplier | int -}}
  
  {{- /* Worker heap is 80% of per-worker memory (leaving 20% for off-heap) */ -}}
  {{- $workerHeapMB := $memPerWorkerMi | float64 | mul 0.8 | int -}}
  
  {{- /* Storm CPU capacity is x100 (e.g., 4 cores = 400) */ -}}
  {{- $supervisorCpuCapacity := $totalCpu | mul 100 | int -}}
  
  {{- dict 
    "containerMemory" (printf "%dMi" $containerMemoryMi)
    "containerCpu" (printf "%g" $totalCpu)
    "supervisorCapacityMB" $totalWorkerMemoryMi
    "supervisorCpuCapacity" $supervisorCpuCapacity
    "workerHeapMB" $workerHeapMB
  -}}
{{- else -}}
  {{- /* Manual mode: Use provided values */ -}}
  {{- $containerMem := required "supervisor.resources.limits.memory is required in manual mode" .Values.supervisor.resources.limits.memory -}}
  {{- $containerCpu := required "supervisor.resources.limits.cpu is required in manual mode" .Values.supervisor.resources.limits.cpu -}}
  {{- $supervisorMemMB := required "supervisor.extraConfig['supervisor.memory.capacity.mb'] is required in manual mode" (index .Values.supervisor.extraConfig "supervisor.memory.capacity.mb") -}}
  {{- $supervisorCpu := default 400 (index .Values.supervisor.extraConfig "supervisor.cpu.capacity") -}}
  {{- $workerHeapMB := default 768 (index .Values.supervisor.extraConfig "worker.heap.memory.mb") -}}
  
  {{- dict 
    "containerMemory" $containerMem
    "containerCpu" $containerCpu
    "supervisorCapacityMB" $supervisorMemMB
    "supervisorCpuCapacity" $supervisorCpu
    "workerHeapMB" $workerHeapMB
  -}}
{{- end -}}
{{- end -}}

{{/*
Validate memory configuration to ensure it's reasonable
*/}}
{{- define "storm.supervisor.validateMemory" -}}
{{- $settings := include "storm.supervisor.memorySettings" . | fromYaml -}}
{{- $slots := .Values.supervisor.slotsPerSupervisor | int -}}

{{- /* Ensure worker heap * slots doesn't exceed supervisor capacity */ -}}
{{- $totalWorkerHeap := mul $settings.workerHeapMB $slots -}}
{{- if gt $totalWorkerHeap $settings.supervisorCapacityMB -}}
  {{- fail printf "Total worker heap (%dMB) exceeds supervisor capacity (%dMB). Reduce slots or increase memory." $totalWorkerHeap $settings.supervisorCapacityMB -}}
{{- end -}}

{{- /* Warn if memory seems too low */ -}}
{{- if lt $settings.workerHeapMB 256 -}}
  {{- fail "Worker heap memory is less than 256MB. This is likely too small for Storm." -}}
{{- end -}}
{{- end -}}

{{/*
Generate Storm configuration entries for supervisor memory
*/}}
{{- define "storm.supervisor.memoryConfig" -}}
{{- $settings := include "storm.supervisor.memorySettings" . | fromYaml -}}
supervisor.memory.capacity.mb: {{ $settings.supervisorCapacityMB }}
supervisor.cpu.capacity: {{ $settings.supervisorCpuCapacity }}
worker.heap.memory.mb: {{ $settings.workerHeapMB }}
{{- end -}}