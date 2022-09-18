package cloudprovider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/date"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/prometheus/azuretracing"

	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
)

type (
	CloudProviderAzure struct {
		CloudProvider

		opts config.Opts
		ctx  context.Context

		log         *log.Entry
		environment azure.Environment
		authorizer  autorest.Authorizer

		keyvaultClient *keyvault.BaseClient
	}
)

func (m *CloudProviderAzure) Init(ctx context.Context, opts config.Opts) {
	var err error
	m.ctx = ctx
	m.opts = opts
	m.log = log.WithField("cloudprovider", "azure")

	if m.opts.CloudProvider.Azure.KeyVaultName == nil || *m.opts.CloudProvider.Azure.KeyVaultName == "" {
		m.log.Panic("no Azure KeyVault name specified")
	}

	if m.opts.CloudProvider.Azure.KeyVaultSecretName == nil || *m.opts.CloudProvider.Azure.KeyVaultSecretName == "" {
		m.log.Panic("no Azure KeyVault secret name specified")
	}

	if m.opts.CloudProvider.Config != nil {
		if err := os.Setenv("AZURE_AUTH_LOCATION", *m.opts.CloudProvider.Config); err != nil {
			m.log.Panic(err)
		}
	}

	// environment
	if m.opts.CloudProvider.Azure.Environment != nil {
		m.environment, err = azure.EnvironmentFromName(*m.opts.CloudProvider.Azure.Environment)
	} else {
		m.environment, err = azure.EnvironmentFromName("AZUREPUBLICCLOUD")
	}
	if err != nil {
		m.log.Panic(err)
	}

	// auth
	if m.opts.CloudProvider.Config != nil {
		m.authorizer, err = auth.NewAuthorizerFromFileWithResource(m.environment.ResourceIdentifiers.KeyVault)
	} else {
		m.authorizer, err = auth.NewAuthorizerFromEnvironmentWithResource(m.environment.ResourceIdentifiers.KeyVault)
	}
	if err != nil {
		m.log.Panic(err)
	}

	// keyvault client
	client := keyvault.New()
	m.decorateAzureAutorestClient(&client.Client)
	m.keyvaultClient = &client
}

func (m *CloudProviderAzure) FetchToken() (token *bootstraptoken.BootstrapToken) {
	vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger := log.WithFields(log.Fields{"keyVault": vaultName, "secretName": secretName})

	contextLogger.Infof("fetching current token from Azure KeyVault \"%s\" secret \"%s\"", vaultName, secretName)
	secret, err := m.keyvaultClient.GetSecret(m.ctx, m.getKeyVaultUrl(), secretName, "")
	if m.handleKeyvaultError(err, contextLogger) != nil {
		contextLogger.Panic(err)
	}

	if secret.Value != nil {
		token = bootstraptoken.ParseFromString(*secret.Value)
		if token != nil {
			if secret.Attributes.Created != nil {
				token.SetCreationUnixTime(*secret.Attributes.Created)
			}

			if secret.Attributes.Expires != nil {
				token.SetExpirationUnixTime(*secret.Attributes.Expires)
			}
		}
	}

	return
}

func (m *CloudProviderAzure) FetchTokens() (tokens []*bootstraptoken.BootstrapToken) {
	tokens = []*bootstraptoken.BootstrapToken{}
	vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger := log.WithFields(log.Fields{"keyVault": vaultName, "secretName": secretName})

	maxResults := int32(15)

	contextLogger.Infof("fetching all tokens from Azure KeyVault \"%s\" secret \"%s\"", vaultName, secretName)
	list, err := m.keyvaultClient.GetSecretVersions(m.ctx, m.getKeyVaultUrl(), secretName, &maxResults)
	if m.handleKeyvaultError(err, contextLogger) != nil {
		contextLogger.Panic(err)
	}

	for _, secret := range list.Values() {
		secretVersion := m.getSecretVersionFromId(*secret.ID)
		secretLogger := contextLogger.WithField("secretVersion", secretVersion)

		if secret.Attributes == nil {
			secretLogger.Debug("ignoring, secret attributes are not set")
			continue
		}

		// ignore not enabled
		if secret.Attributes.Enabled != nil && !*secret.Attributes.Enabled {
			secretLogger.Debug("ignoring, secret is disabled")
			continue
		}

		// ignore expired secrets
		if secret.Attributes.Expires != nil {
			expirationTime := date.UnixEpoch().Add(secret.Attributes.Expires.Duration())
			if expirationTime.Before(time.Now()) {
				secretLogger.Debug("ignoring, secret is expired")
				continue
			}
		}

		if secretVersion != "" {
			secret, err := m.keyvaultClient.GetSecret(m.ctx, m.getKeyVaultUrl(), secretName, secretVersion)
			if m.handleKeyvaultError(err, contextLogger) != nil {
				secretLogger.Panic(err)
			}

			if secret.Value != nil {
				token := bootstraptoken.ParseFromString(*secret.Value)
				if token != nil {
					secretLogger.Info("found valid secret")
					if secret.Attributes.Created != nil {
						token.SetCreationUnixTime(*secret.Attributes.Created)
					}

					if secret.Attributes.Expires != nil {
						token.SetExpirationUnixTime(*secret.Attributes.Expires)
					}

					tokens = append(tokens, token)
				}
			}
		}
	}

	return
}

func (m *CloudProviderAzure) StoreToken(token *bootstraptoken.BootstrapToken) {
	contextLogger := m.log.WithFields(log.Fields{"token": token.Id()})
	vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger.Infof("storing token to Azure KeyVault \"%s\" secret \"%s\" with expiration %s", vaultName, secretName, token.ExpirationString())

	secretParameters := keyvault.SecretSetParameters{
		Value: stringPtr(token.FullToken()),
		Tags: map[string]*string{
			"managed-by": stringPtr("kube-bootstrap-token-manager"),
			"token":      stringPtr(token.Id()),
		},
		ContentType: stringPtr("kube-bootstrap-token"),
		SecretAttributes: &keyvault.SecretAttributes{
			NotBefore: token.CreationUnixTime(),
			Expires:   token.ExpirationUnixTime(),
		},
	}
	_, err := m.keyvaultClient.SetSecret(m.ctx, m.getKeyVaultUrl(), secretName, secretParameters)
	if err != nil {
		log.Panic(err)
	}
}

func (m *CloudProviderAzure) getKeyVaultUrl() (vaultUrl string) {
	vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
	vaultUrl = fmt.Sprintf(
		"https://%s.%s",
		vaultName,
		m.environment.KeyVaultDNSSuffix,
	)

	return
}

func (m *CloudProviderAzure) getSecretVersionFromId(secretId string) (version string) {
	const resourceIDPatternText = `https://(.+)/secrets/(.+)/(.+)`
	resourceIDPattern := regexp.MustCompile(resourceIDPatternText)
	match := resourceIDPattern.FindStringSubmatch(secretId)

	if len(match) == 4 {
		return match[3]
	}

	return ""
}

func (m *CloudProviderAzure) handleKeyvaultError(err error, logger *log.Entry) error {
	if err != nil {
		switch m.getInnerErrorCodeFromAutorestError(err) {
		case "SecretNotFound":
			// no secret found, need to create new token
			logger.Warn("no secret found, assuming non existing token")
		case "SecretDisabled":
			// disabled secret, continue as there would be no token
			logger.Warn("current secret is disabled, assuming non existing token")
		case "ForbiddenByPolicy":
			// access is forbidden
			logger.Error("unable to access Azure KeyVault, please check access")
			return err
		default:
			// not handled error
			return err
		}
	}
	return nil
}

func (m *CloudProviderAzure) getInnerErrorCodeFromAutorestError(err error) (code interface{}) {
	// TODO: check better error handling

	// nolint:errorlint
	if autorestError, ok := err.(autorest.DetailedError); ok {
		// nolint:errorlint
		if azureRequestError, ok := autorestError.Original.(*azure.RequestError); ok {
			if azureRequestError.ServiceError != nil {
				if errorCode, exists := azureRequestError.ServiceError.InnerError["code"]; exists {
					code = errorCode
				}
			}
		}
	}
	return
}

func (m *CloudProviderAzure) decorateAzureAutorestClient(client *autorest.Client) {
	client.Authorizer = m.authorizer
	azuretracing.DecorateAzureAutoRestClient(client)
}
