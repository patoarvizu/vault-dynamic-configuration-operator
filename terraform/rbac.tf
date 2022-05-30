resource kubernetes_cluster_role_v1 cluster_role {
  metadata {
    name = "vault-dynamic-configuration-operator"
  }

  rule {
    verbs      = ["get", "list", "watch"]
    api_groups = [""]
    resources  = ["configmaps"]
  }

  rule {
    verbs      = ["get", "list", "watch"]
    api_groups = [""]
    resources  = ["namespaces"]
  }

  rule {
    verbs      = ["get", "list", "watch"]
    api_groups = [""]
    resources  = ["serviceaccounts"]
  }

  rule {
    verbs      = ["create", "get", "list", "patch", "update", "watch"]
    api_groups = ["vault.banzaicloud.com"]
    resources  = ["vaults"]
  }
}

resource kubernetes_cluster_role_binding_v1 cluster_role_binding {
  metadata {
    name = "vault-dynamic-configuration-operator"
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account_v1.sa.metadata[0].name
    namespace = kubernetes_service_account_v1.sa.metadata[0].namespace
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role_v1.cluster_role.metadata[0].name
  }
}

resource kubernetes_role_v1 role {
  metadata {
    name = "vault-dynamic-configuration-operator"
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name
  }

  rule {
    verbs      = ["get", "list", "watch", "create", "update", "patch", "delete"]
    api_groups = [""]
    resources  = ["configmaps"]
  }

  rule {
    verbs      = ["get", "update", "patch"]
    api_groups = [""]
    resources  = ["configmaps/status"]
  }

  rule {
    verbs      = ["create", "patch"]
    api_groups = [""]
    resources  = ["events"]
  }
}

resource kubernetes_role_binding_v1 rolebinding {
  metadata {
    name = "vault-dynamic-configuration-operator"
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account_v1.sa.metadata[0].name
    namespace = kubernetes_service_account_v1.sa.metadata[0].namespace
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role_v1.role.metadata[0].name
  }
}