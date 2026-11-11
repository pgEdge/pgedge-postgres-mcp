{{/*
Expand the name of the chart.
*/}}
{{- define "pgedge-nla.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "pgedge-nla.fullname" -}}
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
{{- define "pgedge-nla.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pgedge-nla.labels" -}}
helm.sh/chart: {{ include "pgedge-nla.chart" . }}
{{ include "pgedge-nla.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pgedge-nla.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pgedge-nla.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pgedge-nla.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pgedge-nla.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The server's data directory, which is the mount point of the data volume.
*/}}
{{- define "pgedge-nla.dataDir" -}}
/var/lib/pgedge/mcp-server
{{- end }}

{{/*
Whether the server uses the knowledgebase built into its image.
*/}}
{{- define "pgedge-nla.builtInKnowledgebase" -}}
{{- if and .Values.server.knowledgebase.enabled .Values.server.knowledgebase.useBuiltIn }}true{{ end }}
{{- end }}

{{/*
The server image. An empty tag means the chart's appVersion, with the
-with-kb suffix when the built-in knowledgebase is in use.
*/}}
{{- define "pgedge-nla.serverImage" -}}
{{- $tag := .Values.server.image.tag }}
{{- if not $tag }}
{{- $tag = .Chart.AppVersion }}
{{- if include "pgedge-nla.builtInKnowledgebase" . }}
{{- $tag = printf "%s-with-kb" $tag }}
{{- end }}
{{- end }}
{{- printf "%s:%s" .Values.server.image.repository $tag }}
{{- end }}

{{/*
The web client image. An empty tag means the chart's appVersion.
*/}}
{{- define "pgedge-nla.webImage" -}}
{{- printf "%s:%s" .Values.web.image.repository (.Values.web.image.tag | default .Chart.AppVersion) }}
{{- end }}
