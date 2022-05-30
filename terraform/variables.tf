variable service_account_name {
  type = string
  default = "vault-dynamic-configuration-operator"
  description = "The name of the service account to create."
}

variable image_version {
  type = string
  default = "latest"
  description = "The label of the image to run."
}

variable create_namespace {
  type = bool
  default = false
  description = "If true, a new namespace will be created with the name set to the value of the namespace_name variable. If false, it will look up an existing namespace with the name of the value of the namespace_name variable."
}

variable namespace_name {
  type = string
  default = "vault"
  description = "The name of the namespace to create or look up."
}

variable flag_annotation_prefix {
  type = string
  default = "vault.patoarvizu.dev"
  description = "The value of the --annotation-prefix flag"
}

variable flag_target_vault_name {
  type = string
  default = "vault"
  description = "The value of the --target-vault-name flag"
}

variable flag_auto_configure_annotation {
  type = string
  default = "auto-configure"
  description = "The value of the --auto-configure-annotation flag"
}

variable flag_auto_configure_db_creds_annotation {
  type = string
  default = "db-dynamic-creds"
  description = "The value of the --auto-configuredb-creds-annotation flag"
}

variable flag_token_ttl {
  type = string
  default = "5m"
  description = "The value of the --token-ttl flag"
}

variable flag_bound_roles_to_all_namespaces {
  type = bool
  default = false
  description = "The value of the --bound-roles-to-all-namespaces flag"
}

variable watch_namespace {
  type = string
  default = ""
  description = "The value to be set on the `WATCH_NAMESPACE` environment variable."
}

variable enable_prometheus_monitoring {
  type = bool
  default = false
  description = "Create the `Service` and `ServiceMonitor` objects to enable Prometheus monitoring on the operator."
}

variable db_default_ttl {
  type = string
  default = "1h"
  description = "The default value of the `db-default-ttl` setting"
}

variable db_max_ttl {
  type = string
  default = "24h"
  description = "The default value of the `db-max-ttl` setting"
}

variable db_user_creation_statement {
  type = string
  default = "CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT ALL ON *.* TO '{{name}}'@'%';"
  description = "The default value of the `db-user-creation-statement` setting"
}

variable policy_template {
  type = string
  default = <<-EOT
      path "secret/{{ .Name }}" {
        capabilities = ["read"]
      }
      EOT
  description = "The default value of the `policy-template` setting"
}