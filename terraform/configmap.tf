resource kubernetes_config_map_v1 configmap {
  metadata {
    name = "vault-dynamic-configuration"
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name
  }
  data = {
    "policy-template" = var.policy_template
    "db-user-creation-statement" = var.db_user_creation_statement
    "db-default-ttl" = var.db_default_ttl
    "db-max-ttl" = var.db_max_ttl
  }
}