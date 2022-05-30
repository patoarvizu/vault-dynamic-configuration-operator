resource kubernetes_service_account_v1 sa {
  metadata {
    name = var.service_account_name
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name
  }
}