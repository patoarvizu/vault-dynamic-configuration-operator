data kubernetes_namespace ns {
  for_each = var.create_namespace ? {} : {(var.namespace_name): true}
  metadata {
    name = var.namespace_name
  }
}