{{- define "rabbit-jetstream.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "rabbit-jetstream.fullname" -}}
{{- if .Values.fullnameOverride }}{{ .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}{{ else }}{{ printf "%s-%s" .Release.Name (include "rabbit-jetstream.name" .) | trunc 63 | trimSuffix "-" }}{{ end }}
{{- end }}

{{- define "rabbit-jetstream.labels" -}}
app.kubernetes.io/name: {{ include "rabbit-jetstream.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end }}

{{- define "rabbit-jetstream.natsImage" -}}
{{- if .Values.nats.image.digest -}}{{ printf "%s@%s" .Values.nats.image.repository .Values.nats.image.digest }}{{- else -}}{{ printf "%s:%s" .Values.nats.image.repository .Values.nats.image.tag }}{{- end -}}
{{- end }}

{{- define "rabbit-jetstream.managementImage" -}}
{{- if .Values.management.image.digest -}}{{ printf "%s@%s" .Values.management.image.repository .Values.management.image.digest }}{{- else -}}{{ printf "%s:%s" .Values.management.image.repository .Values.management.image.tag }}{{- end -}}
{{- end }}

{{- define "rabbit-jetstream.authSecret" -}}
{{- default (printf "%s-auth" (include "rabbit-jetstream.fullname" .)) .Values.auth.existingSecret }}
{{- end }}

{{- define "rabbit-jetstream.tlsServerSecret" -}}
{{- required "nats.tls.serverSecret is required when nats.tls.enabled=true" .Values.nats.tls.serverSecret -}}
{{- end }}

{{- define "rabbit-jetstream.tlsClientSecret" -}}
{{- required "nats.tls.clientSecret is required when nats.tls.enabled=true" .Values.nats.tls.clientSecret -}}
{{- end }}

{{- define "rabbit-jetstream.monitorURLs" -}}
{{- range $index := until (int .Values.nats.replicaCount) -}}
{{- if $index }},{{ end -}}http://{{ include "rabbit-jetstream.fullname" $ }}-nats-{{ $index }}.{{ include "rabbit-jetstream.fullname" $ }}-nats-headless:8222
{{- end -}}
{{- end }}
