package envoy

import (
	"bytes"
	"text/template"

	"github.com/jxskiss/errors"
)

type BootstrapData struct {
	Cluster  string
	NodeId   string
	XdsProxy string
}

func buildBootstrapConfig(data *BootstrapData) ([]byte, error) {
	var buf bytes.Buffer
	err := bootstrapConfigTmpl.Execute(&buf, data)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return buf.Bytes(), nil
}

var bootstrapConfigTmpl = template.Must(template.New("").Parse(_bcTemplate))

var _bcTemplate = `# This file is auto generated, do not edit.

node:
  cluster: {{ .Cluster }}
  id: {{ .NodeId }}

admin:
  address:
    socket_address:
      address: 127.0.0.1
      port_value: 9000

dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    set_node_on_first_message_only: true
    grpc_services:
      envoy_grpc:
        cluster_name: xds_cluster
  lds_config:
    resource_api_version: V3
    ads: { }
  cds_config:
    resource_api_version: V3
    ads: { }

static_resources:
  clusters:
    - name: xds_cluster
      typed_extension_protocol_options:
        envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
          "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
          explicit_http_config:
            http2_protocol_options: { }
      connect_timeout: 1s
      load_assignment:
        cluster_name: xds_cluster
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    pipe:
                      path: "{{ .XdsProxy }}"
`
