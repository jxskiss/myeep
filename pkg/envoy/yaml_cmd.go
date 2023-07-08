package envoy

import (
	"fmt"
	"strings"

	"github.com/spf13/cast"
)

const cmdPrefix = "!@@ "

func (p *YAMLParser) isCommand(a any) (string, bool) {
	s, ok := a.(string)
	if ok {
		if strings.HasPrefix(s, cmdPrefix) {
			cmd := strings.TrimSpace(s[len(cmdPrefix):])
			return cmd, true
		}
	}
	return "", false
}

func (p *YAMLParser) runCommand(cmd string, arg any) (any, error) {
	switch cmd {
	case "envoy_node":
		return p.cmdEnvoyNode(arg)
	case "envoy_admin":
		return p.cmdEnvoyAdmin(arg)
	case "address":
		return p.cmdAddress(arg)
	case "sds_cluster":
		return p.cmdSDSCluster(arg)
	case "sds_tls":
		return p.cmdDownstreamTlsContext(arg)
	case "http_router":
		return p.cmdHTTPRouter(arg)
	case "acme_challenge":
		return p.cmdACMEChallenge(arg)
	case "redirect_to_https":
		return p.cmdRedirectToHTTPS(arg)
	case "simple_cluster":
		return p.cmdSimpleCluster(arg)
	}
	return nil, fmt.Errorf("unknown command %q with arg %q", cmd, arg)
}

func (p *YAMLParser) cmdEnvoyNode(arg any) (any, error) {
	tmpl := `
node:
  cluster: "{{ .NodeCluster }}"
  id: "{{ .NodeId }}"
`
	return p.parseYAML(tmpl, p.cfg)
}

func (p *YAMLParser) cmdEnvoyAdmin(arg any) (any, error) {
	tmpl := `
admin:
  address:
    socket_address:
      "address": "127.0.0.1"
      "port_value": {{ .AdminPort }}
`
	return p.parseYAML(tmpl, p.cfg)
}

func (p *YAMLParser) cmdAddress(arg any) (any, error) {
	s, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("cmdAddress arg must be string, got %v", arg)
	}

	// Check unix domain socket.
	if strings.HasPrefix(s, "unix:") {
		pipePath := s[5:]
		return map[string]any{
			"address": map[string]any{
				"pipe": map[string]any{
					"path": pipePath,
				},
			},
		}, nil
	}

	// Else it should be an IP:PORT address, other types are not supported.
	parts := strings.SplitN(s, ":", 2)
	return map[string]any{
		"address": map[string]any{
			"socket_address": map[string]any{
				"address":    parts[0],
				"port_value": cast.ToInt(parts[1]),
			},
		},
	}, nil
}

func (p *YAMLParser) cmdSDSCluster(arg any) (any, error) {
	tmpl := `
{{- if .SimpleSSL.Enable }}
name: sds_server_mtls
typed_extension_protocol_options:
  envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
    "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
    explicit_http_config:
      http2_protocol_options:
        connection_keepalive:
          interval: 30s
          timeout: 5s
load_assignment:
  cluster_name: sds_server_mtls
  endpoints:
    - lb_endpoints:
        - endpoint:
            "!@@ address": "{{ .SimpleSSL.SDSAddr }}"
transport_socket:
  name: envoy.transport_sockets.tls
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
    common_tls_context:
      tls_certificates:
        - certificate_chain:
            filename: "{{ .SimpleSSL.ClientCert }}"
          private_key:
            filename: "{{ .SimpleSSL.ClientKey }}"
{{- end }}
`
	return p.parseYAML(tmpl, p.cfg)
}

func (p *YAMLParser) cmdDownstreamTlsContext(arg any) (any, error) {
	tmpl := `
transport_socket:
  name: envoy.transport_sockets.tls
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
    common_tls_context:
      tls_certificate_sds_secret_configs:
        - name: "{{ . }}"
          sds_config:
            resource_api_version: V3
            api_config_source:
              api_type: GRPC
              transport_api_version: V3
              grpc_services:
                envoy_grpc:
                  cluster_name: sds_server_mtls
`
	return p.parseYAML(tmpl, arg)
}

func (p *YAMLParser) cmdHTTPRouter(arg any) (any, error) {
	tmpl := `
name: envoy.filters.http.router
typed_config:
  '@type': type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
`
	return p.parseYAML(tmpl)
}

func (p *YAMLParser) cmdACMEChallenge(arg any) (any, error) {
	if !p.cfg.SimpleSSL.Enable {
		return nil, nil
	}

	tmpl := `
match:
  prefix: "/.well-known/acme-challenge/"
route:
  cluster: simplessl
`
	return p.parseYAML(tmpl)
}

func (p *YAMLParser) cmdRedirectToHTTPS(arg any) (any, error) {
	tmpl := `
match:
  prefix: "/"
redirect:
  path_redirect: "/"
  https_redirect: true
`
	return p.parseYAML(tmpl)
}

func (p *YAMLParser) cmdSimpleCluster(arg any) (any, error) {
	tmpl := `
name: "{{ .name }}"
'@type': type.googleapis.com/envoy.config.cluster.v3.Cluster
connect_timeout: 1s
load_assignment:
  cluster_name: "{{ .name }}"
  endpoints:
    - lb_endpoints:
      {{ range .endpoints }}
      - endpoint:
          "!@@ address": {{ . }}
      {{- end }}
`
	return p.parseYAML(tmpl, arg)
}
