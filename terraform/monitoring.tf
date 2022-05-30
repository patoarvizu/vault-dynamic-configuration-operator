resource kubernetes_service_v1 metrics {
  for_each = var.enable_prometheus_monitoring ? {"monitor": true} : {}
  metadata {
    name = "vault-dynamic-configuration-operator"
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name

    labels = {
      app = "vault-dynamic-configuration-operator"
    }
  }

  spec {
    port {
      name        = "http-metrics"
      protocol    = "TCP"
      port        = 8080
      target_port = "http-metrics"
    }

    selector = {
      app = "vault-dynamic-configuration-operator"
    }

    type = "ClusterIP"
  }
}

resource kubernetes_manifest servicemonitor_metrics {
  for_each = var.enable_prometheus_monitoring ? {"monitor": true} : {}
  manifest = {
    apiVersion = "monitoring.coreos.com/v1"
    kind = "ServiceMonitor"
    metadata = {
      name = "vault-dynamic-configuration-operator"
      namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name
    }
    spec = {
      endpoints = [
        {
          path = "/metrics"
          port = "http-metrics"
        },
      ]
      selector = {
        matchLabels = {
          app = "vault-dynamic-configuration-operator"
        }
      }
    }
  }
}