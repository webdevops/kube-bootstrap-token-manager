package cloudprovider

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"go.uber.org/zap"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/webdevops/go-common/azuresdk/armclient"

	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
)

const (
	SECRET_SYNC_COUNT_MAX = 15
)

type (
	CloudProviderAzure struct {
		CloudProvider

		opts config.Opts
		ctx  context.Context

		logger *zap.SugaredLogger
		client *armclient.ArmClient

		keyvaultClient *azsecrets.Client
	}
)

func (m *CloudProviderAzure) Init(ctx context.Context, opts config.Opts, logger *zap.SugaredLogger, userAgent string) {
	var err error
	m.ctx = ctx
	m.opts = opts
	m.logger = logger.With(
		zap.String("cloudprovider", "azure"),
	)

	m.client, err = armclient.NewArmClientFromEnvironment(logger)
	if err != nil {
		logger.Fatal(err.Error())
	}
	m.client.SetUserAgent(userAgent)

	if m.opts.CloudProvider.Azure.KeyVaultUrl == nil || *m.opts.CloudProvider.Azure.KeyVaultUrl == "" {
		m.logger.Panic("no Azure KeyVault name specified")
	}

	if m.opts.CloudProvider.Azure.KeyVaultSecretName == nil || *m.opts.CloudProvider.Azure.KeyVaultSecretName == "" {
		m.logger.Panic("no Azure KeyVault secret name specified")
	}

	// keyvault client
	secretOpts := azsecrets.ClientOptions{
		ClientOptions: *m.client.NewAzCoreClientOptions(),
	}
	m.keyvaultClient, err = azsecrets.NewClient(*m.opts.CloudProvider.Azure.KeyVaultUrl, m.client.GetCred(), &secretOpts)
	if err != nil {
		m.logger.Panic(err)
	}
}

func (m *CloudProviderAzure) FetchToken() (token *bootstraptoken.BootstrapToken) {
	vaultUrl := *m.opts.CloudProvider.Azure.KeyVaultUrl
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger := m.logger.With(zap.String("keyVault", vaultUrl), zap.String("secretName", secretName))

	contextLogger.Infof("fetching current token from Azure KeyVault \"%s\" secret \"%s\"", vaultUrl, secretName)
	secret, err := m.keyvaultClient.GetSecret(m.ctx, secretName, "", nil)
	if m.handleKeyvaultError(contextLogger, err) != nil {
		contextLogger.Panic(err)
	}

	if secret.Value != nil {
		token = bootstraptoken.ParseFromString(*secret.Value)
		if token != nil {
			if secret.Attributes.Created != nil {
				token.SetCreationTime(*secret.Attributes.Created)
			}

			if secret.Attributes.Expires != nil {
				token.SetExpirationTime(*secret.Attributes.Expires)
			}

			m.updateTokenMeta(token, secret)

			token.SetAnnotation("bootstraptoken.webdevops.io/provider", "azure")
			token.SetAnnotation("bootstraptoken.webdevops.io/keyvault", vaultUrl)
			token.SetAnnotation("bootstraptoken.webdevops.io/secret", secretName)
			token.SetAnnotation("bootstraptoken.webdevops.io/secretVersion", secret.ID.Version())
		}
	}

	return
}

func (m *CloudProviderAzure) FetchTokens() (tokens []*bootstraptoken.BootstrapToken) {
	tokens = []*bootstraptoken.BootstrapToken{}
	vaultUrl := *m.opts.CloudProvider.Azure.KeyVaultUrl
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger := m.logger.With(zap.String("keyVault", vaultUrl), zap.String("secretName", secretName))

	contextLogger.Infof("fetching all tokens from Azure KeyVault \"%s\" secret \"%s\"", vaultUrl, secretName)

	pager := m.keyvaultClient.NewListSecretPropertiesVersionsPager(secretName, nil)
	// get secrets first
	secretCandidateList := []*azsecrets.SecretProperties{}
	for pager.More() {
		result, err := pager.NextPage(m.ctx)
		if err != nil {
			m.logger.Panic(err)
		}

		for _, secretVersion := range result.Value {
			if !*secretVersion.Attributes.Enabled {
				continue
			}

			if secretVersion.Attributes.NotBefore != nil && time.Now().Before(*secretVersion.Attributes.NotBefore) {
				// not yet valid
				continue
			}

			if secretVersion.Attributes.Expires != nil && time.Now().After(*secretVersion.Attributes.Expires) {
				// expired
				continue
			}

			secretCandidateList = append(secretCandidateList, secretVersion)
		}
	}

	// sort results
	sort.Slice(secretCandidateList, func(i, j int) bool {
		return secretCandidateList[i].Attributes.Created.UTC().After(secretCandidateList[j].Attributes.Created.UTC())
	})

	// process list
	secretCounter := 0
	for _, secretVersion := range secretCandidateList {
		secretLogger := contextLogger.With(zap.String("secretVersion", secretVersion.ID.Version()))

		secret, err := m.keyvaultClient.GetSecret(m.ctx, secretVersion.ID.Name(), secretVersion.ID.Version(), nil)
		if err != nil {
			secretLogger.Warn(`unable to fetch secret "%[2]v" with version "%[3]v" from vault "%[1]v": %[4]w`, vaultUrl, secretVersion.ID.Name(), secretVersion.ID.Version(), err)
			continue
		}

		if secret.Value != nil {
			token := bootstraptoken.ParseFromString(*secret.Value)
			if token != nil {
				secretLogger.Info("found valid secret")

				if secret.Attributes.Created != nil {
					token.SetCreationTime(*secret.Attributes.Created)
				}

				if secret.Attributes.Expires != nil {
					token.SetExpirationTime(*secret.Attributes.Expires)
				}

				m.updateTokenMeta(token, secret)

				tokens = append(tokens, token)
			}
		}

		secretCounter++
		if secretCounter > SECRET_SYNC_COUNT_MAX {
			break
		}
	}

	return
}

func (m *CloudProviderAzure) StoreToken(token *bootstraptoken.BootstrapToken) {
	contextLogger := m.logger.With(zap.String("token", token.Id()))
	vaultUrl := *m.opts.CloudProvider.Azure.KeyVaultUrl
	secretName := *m.opts.CloudProvider.Azure.KeyVaultSecretName

	contextLogger.Infof("storing token to Azure KeyVault \"%s\" secret \"%s\" with expiration %s", vaultUrl, secretName, token.ExpirationString())

	secretParameters := azsecrets.SetSecretParameters{
		Value: stringPtr(token.FullToken()),
		Tags: map[string]*string{
			"managed-by": stringPtr("kube-bootstrap-token-manager"),
			"token":      stringPtr(token.Id()),
		},
		ContentType: stringPtr("kube-bootstrap-token"),
		SecretAttributes: &azsecrets.SecretAttributes{
			NotBefore: token.CreationTime(),
			Expires:   token.ExpirationTime(),
		},
	}

	_, err := m.keyvaultClient.SetSecret(m.ctx, secretName, secretParameters, nil)
	if err != nil {
		m.logger.Panic(err)
	}
}

func (m *CloudProviderAzure) updateTokenMeta(token *bootstraptoken.BootstrapToken, secret azsecrets.GetSecretResponse) {
	token.SetAnnotation("bootstraptoken.webdevops.io/provider", "azure")
	token.SetAnnotation("bootstraptoken.webdevops.io/keyvault", *m.opts.CloudProvider.Azure.KeyVaultUrl)
	token.SetAnnotation("bootstraptoken.webdevops.io/secret", secret.ID.Name())
	token.SetAnnotation("bootstraptoken.webdevops.io/secretVersion", secret.ID.Version())

	if secret.Attributes.Created != nil {
		token.SetAnnotation("bootstraptoken.webdevops.io/created", secret.Attributes.Created.Format(time.RFC3339))
	}

	if secret.Attributes.Expires != nil {
		token.SetAnnotation("bootstraptoken.webdevops.io/expires", secret.Attributes.Expires.Format(time.RFC3339))
	}

	if secret.Attributes.NotBefore != nil {
		token.SetAnnotation("bootstraptoken.webdevops.io/notBefore", secret.Attributes.NotBefore.Format(time.RFC3339))
	}
}

func (m *CloudProviderAzure) handleKeyvaultError(logger *zap.SugaredLogger, err error) error {
	if err != nil {
		switch m.parseAzCoreResponseError(err) {
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

func (m *CloudProviderAzure) parseAzCoreResponseError(err error) (code interface{}) {
	// TODO: check better error handling

	// nolint:errorlint
	var responseError *azcore.ResponseError
	if err != nil && errors.As(err, &responseError) {
		// nolint:errorlint
		code = responseError.ErrorCode
	}
	return
}
