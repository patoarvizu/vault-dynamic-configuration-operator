resource kubernetes_deployment_v1 deployment {
  metadata {
    name = "vault-dynamic-configuration-operator"
    namespace = var.create_namespace ? kubernetes_namespace_v1.ns[var.namespace_name].metadata[0].name : data.kubernetes_namespace.ns[var.namespace_name].metadata[0].name

    labels = {
      app = "vault-dynamic-configuration-operator"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "vault-dynamic-configuration-operator"
      }
    }

    template {
      metadata {
        labels = {
          app = "vault-dynamic-configuration-operator"
        }
      }

      spec {
        service_account_name = kubernetes_service_account_v1.sa.metadata[0].name

        container {
          name    = "manager"
          image   = "patoarvizu/vault-dynamic-configuration-operator:${var.image_version}"
          image_pull_policy = "IfNotPresent"
          command = ["/manager"]
          args    = [
            "--enable-leader-election",
            "--annotation-prefix=${var.flag_annotation_prefix}",
            "--target-vault-name=${var.flag_annotation_prefix}",
            "--auto-configure-annotation=${var.flag_auto_configure_annotation}",
            "--auto-configuredb-creds-annotation=${var.flag_auto_configure_db_creds_annotation}",
            "--token-ttl=${var.flag_token_ttl}",
            "--bound-roles-to-all-namespaces=${tostring(var.flag_bound_roles_to_all_namespaces)}"
          ]

          port {
            name           = "http-metrics"
            container_port = 8080
          }

          env {
            name = "WATCH_NAMESPACE"
            value = var.watch_namespace
          }
        }
      }
    }
  }
}

