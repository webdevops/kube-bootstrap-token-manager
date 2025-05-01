# Kubernetes node bootstrap token manager

[![license](https://img.shields.io/github/license/webdevops/kube-bootstrap-token-manager.svg)](https://github.com/webdevops/kube-bootstrap-token-manager/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fkube--bootstrap--token--manager-blue)](https://hub.docker.com/r/webdevops/kube-bootstrap-token-manager/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fkube--bootstrap--token--manager-blue)](https://quay.io/repository/webdevops/kube-bootstrap-token-manager)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kube-bootstrap-token-manager)](https://artifacthub.io/packages/search?repo=kube-bootstrap-token-manager)

Manager for Node bootstrap tokens for Kubernetes.

Supports currently Azure cloud provider (more cloud provider support -> please submit PR).

Azure:
- Stores token in Keyvault as secret
- (re)creates token inside Kubernetes and ensures it existence
- Manages renewal if token is going to be expired

## Configuration

```
Usage:
  kube-bootstrap-token-manager [OPTIONS]

Application Options:
      --log.debug                                      debug mode [$LOG_DEBUG]
      --log.devel                                      development mode [$LOG_DEVEL]
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
      --azure.keyvault.url=                            URL of Keyvault to sync token [$AZURE_KEYVAULT_URL]
      --azure.keyvault.secret=                         Name of Keyvault secret to sync token (default: kube-bootstrap-token) [$AZURE_KEYVAULT_SECRET]
      --dry-run                                        Dry run (do not apply to nodes) [$DRY_RUN]
      --server.bind=                                   Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=                           Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=                          Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                                           Show this help message
```

for Azure API authentication (using ENV vars) see following documentations:
- https://github.com/webdevops/go-common/blob/main/azuresdk/README.md
- https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication

## Metrics

 (see `:8080/metrics`)

| Metric                             | Description                                     |
|:-----------------------------------|:------------------------------------------------|
| `bootstraptoken_token_info`        | Info about current token                        |
| `bootstraptoken_token_expiration`  | Expiration time (unix timestamp) of token       |
| `bootstraptoken_sync_status`       | Status if sync was successfull                  |
| `bootstraptoken_sync_time`         | Timestamp of last sync                          |
| `bootstraptoken_sync_count`        | Counter of sync                                 |

### AzureTracing metrics

see [armclient tracing documentation](https://github.com/webdevops/go-common/blob/main/azuresdk/README.md#azuretracing-metrics)

#### Settings

| Environment variable                     | Example                            | Description                                                    |
|------------------------------------------|------------------------------------|----------------------------------------------------------------|
| `METRIC_AZURERM_API_REQUEST_BUCKETS`     | `1, 2.5, 5, 10, 30, 60, 90, 120`   | Sets buckets for `azurerm_api_request` histogram metric        |
| `METRIC_AZURERM_API_REQUEST_ENABLE`      | `false`                            | Enables/disables `azurerm_api_request_*` metric                |
| `METRIC_AZURERM_API_REQUEST_LABELS`      | `apiEndpoint, method, statusCode`  | Controls labels of `azurerm_api_request_*` metric              |
| `METRIC_AZURERM_API_RATELIMIT_ENABLE`    | `false`                            | Enables/disables `azurerm_api_ratelimit` metric                |
| `METRIC_AZURERM_API_RATELIMIT_AUTORESET` | `false`                            | Enables/disables `azurerm_api_ratelimit` autoreset after fetch |


| `azurerm_api_request` label | Status             | Description                                                                                              |
|-----------------------------|--------------------|----------------------------------------------------------------------------------------------------------|
| `apiEndpoint`               | enabled by default | hostname of endpoint (max 3 parts)                                                                       |
| `routingRegion`             | enabled by default | detected region for API call, either routing region from Azure Management API or Azure resource location |
| `subscriptionID`            | enabled by default | detected subscriptionID                                                                                  |
| `tenantID`                  | enabled by default | detected tenantID (extracted from jwt auth token)                                                        |
| `resourceProvider`          | enabled by default | detected Azure Management API provider                                                                   |
| `method`                    | enabled by default | HTTP method                                                                                              |
| `statusCode`                | enabled by default | HTTP status code                                                                                         |


## Kubernetes deployment

see [deployment](/deployment)
