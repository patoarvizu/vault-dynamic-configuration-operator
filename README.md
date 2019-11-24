# Vault dynamic configuration Operator

![CircleCI](https://img.shields.io/circleci/build/github/patoarvizu/vault-dynamic-configuration-operator.svg?label=CircleCI) ![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/patoarvizu/vault-dynamic-configuration-operator.svg) ![Docker Pulls](https://img.shields.io/docker/pulls/patoarvizu/vault-dynamic-configuration-operator.svg) ![Keybase BTC](https://img.shields.io/keybase/btc/patoarvizu.svg) ![Keybase PGP](https://img.shields.io/keybase/pgp/patoarvizu.svg) ![GitHub](https://img.shields.io/github/license/patoarvizu/vault-dynamic-configuration-operator.svg)

<!-- TOC -->

- [Vault dynamic configuration Operator](#vault-dynamic-configuration-operator)
    - [Intro](#intro)
    - [Auto-configure roles and policies](#auto-configure-roles-and-policies)
    - [Auto-configure dynamic database credentials](#auto-configure-dynamic-database-credentials)
    - [Configuration](#configuration)
        - [Operator command-line flags](#operator-command-line-flags)
        - [Operator permissions](#operator-permissions)
    - [For security nerds](#for-security-nerds)
        - [Docker images are signed and published to Docker Hub's Notary server](#docker-images-are-signed-and-published-to-docker-hubs-notary-server)
        - [Docker images are labeled with Git and GPG metadata](#docker-images-are-labeled-with-git-and-gpg-metadata)
    - [Notes](#notes)
    - [Help wanted!](#help-wanted)

<!-- /TOC -->

## Intro

The [Bank Vaults Operator](https://github.com/banzaicloud/bank-vaults/tree/master/operator) provides a powerful and useful abstraction for managing a Vault cluster in Kubernetes. However, one thing it lacks is a way to automate changes to the configuration on a per-service basis.

The purpose of this operator is to provide a mechanism to automatically add individual services' configuration (roles and policies) based on annotations added to `ServiceAccount`s.

## Auto-configure roles and policies

The operator will listen for `ServiceAccount` objects and add a Kubernetes [role](https://www.vaultproject.io/api/auth/kubernetes/index.html#create-role) to the Vault auth configuration, and attach to it the configured policy (or rendered policy template).

Note that this operator doesn't enforce that the annotated `ServiceAccount` is attached to any specific workload (`Pod`, `Deployment`, `StatefulSet`, etc.), that enforcement should come from another source, like an [Admission Controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) or [Open Policy Agent](https://www.openpolicyagent.org/).

## Auto-configure dynamic database credentials

Additionally, if the service account is annotated with `vault.patoarvizu.dev/db-dynamic-creds` (or the custom values, if overwritten on the command line), the operator will add a [role](https://www.vaultproject.io/api/secret/databases/index.html#create-role) for dynamic database credentials. One or more database [connections](https://www.vaultproject.io/api/secret/databases/index.html#create-role) should be previously configured with the appropriate credentials.

The operator will take the value of the annotation and create a new role for the database connection with that name, and add the service name as an [allowed role](https://www.vaultproject.io/api/secret/databases/index.html#allowed_roles). All new roles will be created using the values of `db-user-creation-statement`, `db-default-ttl`, and `db-max-ttl` from the `vault-dynamic-configuration` `ConfigMap`.

As of this version, only [MySQL/MariaDB](https://www.vaultproject.io/api/secret/databases/mysql-maria.html) dynamic credentials are supported.

## Configuration

### Operator command-line flags

Flag | Description | Default
---------|----------|---------
 `--target-vault-name` | Name of the Bank-Vaults CRD to target for modifications. The CRD must be deployed in the same namespace as the operator. | `vault`
 `--annotation-prefix` | The prefix to all annotations used and discovered by the controller. | `vault.patoarvizu.dev`
 `--auto-configure-annotation` | The annotation that must be appended to the `--annotation-prefix` value (with a `/` as a separator between the two) and added to `ServiceAccount` objects to automatically configure it for Vault access. The value of the annotation must be `"true"`, any other value will be ignored. | `auto-configure`
 `--dynamic-db-credentials-annotation` | The annotation that must be appended to the `--annotation-prefix` value (with a `/` as a separator between the two) and added to `ServiceAccount` objects to automatically configure it for having access to generate dynamic database credentials. The value of the annotation must be `"true"`, any other value will be ignored. | `db-dynamic-creds`
 `--bound-roles-to-all-namespaces` | Set `bound_service_account_namespaces` to `'*'` instead of the service account's namespace. | `false`
 `--token-ttl` | Value to set roles' `token_ttl` to | `5m`

 ### ConfigMap

In addition to the command-line flags, this operator also reads configuration from a `ConfigMap` called `vault-dynamic-configuration`. Any changes made to the `ConfigMap` are automatically picked up and applied to the target Vault configuration.

Field | Description
---------|----------
`policy-template` | A [Go template](https://golang.org/pkg/text/template/) that will be rendered into the full policy to be attached to each service account/role. The only two available values are `.Name` and `.Namespace`.

### Operator permissions

Since the operator is **not** operating on the Vault cluster directly, it doesn't need to authenticate itself against it. However, it should run with a service account with enough permissions to perform the required actions against the Kubernetes API, including the modification of Vault CRD objects.

## For security nerds

### Docker images are signed and published to Docker Hub's Notary server

The [Notary](https://github.com/theupdateframework/notary) project is a CNCF incubating project that aims to provide trust and security to software distribution. Docker Hub runs a Notary server at https://notary.docker.io for the repositories it hosts.

[Docker Content Trust](https://docs.docker.com/engine/security/trust/content_trust/) is the mechanism used to verify digital signatures and enforce security by adding a validating layer.

You can inspect the signed tags for this project by doing `docker trust inspect --pretty docker.io/patoarvizu/vault-dynamic-configuration-operator`, or (if you already have `notary` installed) `notary -d ~/.docker/trust/ -s https://notary.docker.io list docker.io/patoarvizu/vault-dynamic-configuration-operator`.

If you run `docker pull` with `DOCKER_CONTENT_TRUST=1`, the Docker client will only pull images that come from registries that have a Notary server attached (like Docker Hub).

### Docker images are labeled with Git and GPG metadata

In addition to the digital validation done by Docker on the image itself, you can do your own human validation by making sure the image's content matches the Git commit information (including tags if there are any) and that the GPG signature on the commit matches the key on the commit on github.com.

For example, if you run `docker pull patoarvizu/vault-dynamic-configuration-operator:4773304232a88cc495a1d896f59a0cff3a6faa46` to pull the image tagged with that commit id, then run `docker inspect patoarvizu/vault-dynamic-configuration-operator:4773304232a88cc495a1d896f59a0cff3a6faa46 | jq -r '.[0].ContainerConfig.Labels'` (assuming you have [jq](https://stedolan.github.io/jq/) installed) you should see that the `GIT_COMMIT` label matches the tag on the image. Furthermore, if you go to https://github.com/patoarvizu/vault-dynamic-configuration-operator/commit/4773304232a88cc495a1d896f59a0cff3a6faa46 (notice the matching commit id), and click on the **Verified** button, you should be able to confirm that the GPG key ID used to match this commit matches the value of the `SIGNATURE_KEY` label, and that the key belongs to the `AUTHOR_EMAIL` label. When an image belongs to a commit that was tagged, it'll also include a `GIT_TAG` label, to further validate that the image matches the content.

Keep in mind that this isn't tamper-proof. A malicious actor with access to publish images can create one with malicious content but with values for the labels matching those of a valid commit id. However, when combined with Docker Content Trust, the certainty of using a legitimate image is increased because the chances of a bad actor having both the credentials for publishing images, as well as Notary signing credentials are significantly lower and even in that scenario, compromised signing keys can be revoked or rotated.

Here's the list of included Docker labels:

- `AUTHOR_EMAIL`
- `COMMIT_TIMESTAMP`
- `GIT_COMMIT`
- `GIT_TAG`
- `SIGNATURE_KEY`

## Notes

* If the annotation is added to a service account that matches a role/policy that already exists in the Vault CRD will be modified, but all other role/policies will be kept as they are defined.
* Currently, the Operator will add the appropriate configuration, but won't remove it if the annotation is removed (or set to a non-`true` value), or if the service account itself is removed.

## Help wanted!

All Issues or PRs on this repo are welcome, even if it's for a typo or an open-ended question.