Kubernetes node bootstrap token manager
========================================

[![license](https://img.shields.io/github/license/webdevops/kube-bootstrap-token-manager.svg)](https://github.com/webdevops/kube-bootstrap-token-manager/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/cloud/automated/webdevops/kube-bootstrap-token-manager)](https://hub.docker.com/r/webdevops/kube-bootstrap-token-manager/)
[![Docker Build Status](https://img.shields.io/docker/cloud/build/webdevops/kube-bootstrap-token-manager)](https://hub.docker.com/r/webdevops/kube-bootstrap-token-manager/)

Manager for Node bootstrap tokens for Kubernetes.

Supports currently Azure cloud provider (more cloud provider support -> please submit PR).

Azure:
- Stores token in Keyvault as secret
- (re)creates token inside Kubernetes and ensures it existence
- Manages renewal if token is going to be expired

Configuration
-------------

```
Usage:
  kube-bootstrap-token-manager [OPTIONS]

Application Options:
      --debug                                          debug mode [$DEBUG]
      --trace                                          verbose mode [$TRACE]
      --log.json                                       Switch log output to json format [$LOG_JSON]
      --bootstraptoken.id-template=                    Template for token ID for bootstrap tokens (default: {{.Date}}) [$BOOTSTRAPTOKEN_ID_TEMPLATE]
      --bootstraptoken.name=                           Name for bootstrap tokens (default: bootstrap-token-%s) [$BOOTSTRAPTOKEN_NAME]
      --bootstraptoken.label=                          Label for bootstrap tokens (default: webdevops.kubernetes.io/bootstraptoken-managed) [$BOOTSTRAPTOKEN_LABEL]
      --bootstraptoken.namespace=                      Namespace for bootstrap tokens (default: kube-system) [$BOOTSTRAPTOKEN_NAMESPACE]
      --bootstraptoken.type=                           Type for bootstrap tokens (default: bootstrap.kubernetes.io/token) [$BOOTSTRAPTOKEN_TYPE]
      --bootstraptoken.usage-bootstrap-authentication= Usage bootstrap authentication for bootstrap tokens (default: true) [$BOOTSTRAPTOKEN_USAGE_BOOTSTRAP_AUTHENTICATION]
      --bootstraptoken.usage-bootstrap-signing=        usage bootstrap signing for bootstrap tokens (default: true) [$BOOTSTRAPTOKEN_USAGE_BOOTSTRAP_SIGNING]
      --bootstraptoken.auth-extra-groups=              Auth extra groups for bootstrap tokens (default: system:bootstrappers:worker,system:bootstrappers:ingress) [$BOOTSTRAPTOKEN_AUTH_EXTRA_GROUPS]
      --bootstraptoken.expiration=                     Expiration (time.Duration) for bootstrap tokens (default: 8760h) [$BOOTSTRAPTOKEN_EXPIRATION]
      --bootstraptoken.token-length=                   Length of the random token string for bootstrap tokens (default: 16) [$BOOTSTRAPTOKEN_TOKEN_LENGTH]
      --bootstraptoken.token-runes=                    Runes which should be used for the random token string for bootstrap tokens (default: abcdefghijklmnopqrstuvwxyz0123456789)
                                                       [$BOOTSTRAPTOKEN_TOKEN_RUNES]
      --sync.time=                                     Sync time (time.Duration) (default: 1h) [$SYNC_TIME]
      --sync.recreate-before=                          Time duration (time.Duration) when token should be recreated (default: 2190h) [$SYNC_RECREATE_BEFORE]
      --sync.full                                      Sync also previous tokens (full sync) [$SYNC_FULL]
      --cloud-provider=[azure]                         Cloud provider [$CLOUD_PROVIDER]
      --cloud-config=                                  Cloud provider configuration path [$CLOUD_CONFIG]
      --azure-environment=                             Azure environment name [$AZURE_ENVIRONMENT]
      --azure.keyvault-name=                           Name of Keyvault to sync token [$AZURE_KEYVAULT_NAME]
      --azure.keyvault-secret-name=                    Name of Keyvault secret to sync token (default: kube-bootstrap-token) [$AZURE_KEYVAULT_SECRET_NAME]
      --dry-run                                        Dry run (do not apply to nodes) [$DRY_RUN]
      --bind=                                          Server address (default: :8080) [$SERVER_BIND]

Help Options:
  -h, --help                                           Show this help message
```

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

 (see `:8080/metrics`)

| Metric                             | Description                                     |
|:-----------------------------------|:------------------------------------------------|
| `bootstraptoken_token_info`        | Info about current token                        |
| `bootstraptoken_token_expiration`  | Expiration time (unix timestamp) of token       |
| `bootstraptoken_sync_status`       | Status if sync was successfull                  |
| `bootstraptoken_sync_time`         | Timestamp of last sync                          |
| `bootstraptoken_sync_count`        | Counter of sync                                 |

Kubernetes deployment
---------------------

see [deployment](/deployment)
