package cloudprovider

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
	"os"
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

	if m.opts.CloudProvider.Config != nil {
		os.Setenv("AZURE_AUTH_LOCATION", *m.opts.CloudProvider.Config)
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
	client.Authorizer = m.authorizer
	m.keyvaultClient = &client
}

func (m *CloudProviderAzure) FetchToken() (token *bootstraptoken.BootstrapToken) {
	if m.opts.CloudProvider.Azure.KeyVaultName != nil && m.opts.CloudProvider.Azure.KeyVaultSecretName != nil {
		vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
		secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName
		vaultUrl := fmt.Sprintf(
			"https://%s.%s",
			vaultName,
			m.environment.KeyVaultDNSSuffix,
		)

		log.Infof("fetching newest token from Azure KeyVault \"%s\" secret \"%s\"", vaultName, secretName)
		secret, err := m.keyvaultClient.GetSecret(m.ctx, vaultUrl, secretName, "")
		// ignore if not found as "non error"
		if !secret.IsHTTPStatus(404) && err != nil {
			log.Panic(err)
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
	}

	if token != nil {
		contextLogger := log.WithFields(log.Fields{"token": token.Id()})
		contextLogger.Infof("found cloud token with id \"%s\" and expiration %s", token.Id(), token.ExpirationString())
	} else {
		log.Infof("no cloud token found")
	}

	return
}

func (m *CloudProviderAzure) StoreToken(token *bootstraptoken.BootstrapToken) {
	contextLogger := m.log.WithFields(log.Fields{"token": token.Id()})
	if m.opts.CloudProvider.Azure.KeyVaultName != nil && m.opts.CloudProvider.Azure.KeyVaultSecretName != nil {
		vaultName := *m.opts.CloudProvider.Azure.KeyVaultName
		secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName
		vaultUrl := fmt.Sprintf(
			"https://%s.%s",
			vaultName,
			m.environment.KeyVaultDNSSuffix,
		)

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
		_, err := m.keyvaultClient.SetSecret(m.ctx, vaultUrl, secretName, secretParameters)
		if err != nil {
			log.Panic(err)
		}
	}
}
