# Vault dynamic configuration Operator

<!-- TOC -->

- [Vault dynamic configuration Operator](#vault-dynamic-configuration-operator)
    - [Intro](#intro)
    - [Configuration](#configuration)
        - [Operator command-line flags](#operator-command-line-flags)
        - [Operator permissions](#operator-permissions)
    - [Notes](#notes)

<!-- /TOC -->

## Intro

The [Bank Vaults Operator](https://github.com/banzaicloud/bank-vaults/tree/master/operator) provides a powerful and useful abstraction for managing a Vault cluster in Kubernetes. However, one thing it lacks is a way to automate changes to the configuration on a per-service basis.

The purpose of this operator is to provide a mechanism to automatically add individual services' configuration (roles and policies) based on annotations added to `ServiceAccount`s.

## Configuration

### Operator command-line flags

Flag | Description | Default
---------|----------|---------
 `--target-vault-name` | Name of the Bank-Vaults CRD to target for modifications. The CRD must be deployed in the same namespace as the operator. | `vault`
 `--auto-configure-annotation` | The annotation that must be added to `ServiceAccount` objects to automatically configure it for Vault access. The value of the annotation must be `"true"`, any other value will be ignored. | `vault.patoarvizu.dev/auto-configure`
 `--bound-roles-to-all-namespaces` | Set `bound_service_account_namespaces` to `'*'` instead of the service account's namespace. | `false`
 `--token-ttl` | Value to set roles' `token_ttl` to | `5m`

 ### ConfigMap

In addition to the command-line flags, this operator also reads configuration from a `ConfigMap` called `vault-dynamic-configuration`. Any changes made to the `ConfigMap` are automatically picked up and applied to the target Vault configuration.

Field | Description
---------|----------
`policy-template` | A [Go template](https://golang.org/pkg/text/template/) that will be rendered into the full policy to be attached to each service account/role. The only two available values are `.Name` and `.Namespace`.

### Operator permissions

Since the operator is **not** operating on the Vault cluster directly, it doesn't need to authenticate itself against it. However, it should run with a service account with enough permissions to perform the required actions against the Kubernetes API, including the modification of Vault CRD objects.

## Notes

* If the annotation is added to a service account that matches a role/policy that already exists in the Vault CRD will be modified, but all other role/policies will be kept as they are defined.
* Currently, the Operator will add the appropriate configuration, but won't remove it if the annotation is removed (or set to a non-`true` value), or if the service account itself is removed.