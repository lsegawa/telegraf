# Jolokia2 Proxy Input Plugin

The `jolokia2_proxy` input plugin reads JMX metrics from one or more _targets_
by interacting with a [Jolokia proxy](https://jolokia.org/features/proxy.html)
REST endpoint.

## Configuration

```toml @sample.conf
# Read JMX metrics from a Jolokia REST proxy endpoint
[[inputs.jolokia2_proxy]]
  # default_tag_prefix      = ""
  # default_field_prefix    = ""
  # default_field_separator = "."

  ## Proxy agent
  url = "http://localhost:8080/jolokia"
  # username = ""
  # password = ""
  # response_timeout = "5s"

  ## Optional origin URL to include as a header in the request. Some endpoints
  ## may reject an empty origin.
  # origin = ""

  ## Optional TLS config
  # tls_ca   = "/var/private/ca.pem"
  # tls_cert = "/var/private/client.pem"
  # tls_key  = "/var/private/client-key.pem"
  # insecure_skip_verify = false

  ## Add proxy targets to query
  # default_target_username = ""
  # default_target_password = ""
  [[inputs.jolokia2_proxy.target]]
    url = "service:jmx:rmi:///jndi/rmi://targethost:9999/jmxrmi"
    # username = ""
    # password = ""

  ## Add metrics to read
  [[inputs.jolokia2_proxy.metric]]
    name  = "java_runtime"
    mbean = "java.lang:type=Runtime"
    paths = ["Uptime"]
```
