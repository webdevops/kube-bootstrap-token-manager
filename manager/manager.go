package manager

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"math/rand"
	"os"
	"time"
)

type (
	KubeBootstrapTokenManager struct {
		Opts config.Opts

		ctx       context.Context
		k8sClient *kubernetes.Clientset

		prometheus struct {
			token           *prometheus.GaugeVec
			tokenExpiration *prometheus.GaugeVec

			sync      *prometheus.GaugeVec
			syncTime  *prometheus.GaugeVec
			syncCount *prometheus.CounterVec
		}

		cloud struct {
			azure struct {
				environment azure.Environment
				authorizer  autorest.Authorizer

				keyvaultClient *keyvault.BaseClient
			}
		}
	}
)

func (m *KubeBootstrapTokenManager) Init() {
	m.ctx = context.Background()
	rand.Seed(time.Now().UnixNano())
	m.initK8s()
	m.initPrometheus()
	m.initCloudProvider()
}

func (m *KubeBootstrapTokenManager) initPrometheus() {
	m.prometheus.token = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootstraptoken_token_info",
			Help: "kube-bootstrap-token-manager token info",
		},
		[]string{"tokenID"},
	)
	prometheus.MustRegister(m.prometheus.token)

	m.prometheus.tokenExpiration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootstraptoken_token_expiration",
			Help: "kube-bootstrap-token-manager token expiration time",
		},
		[]string{"tokenID"},
	)
	prometheus.MustRegister(m.prometheus.tokenExpiration)

	m.prometheus.sync = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootstraptoken_sync_status",
			Help: "kube-bootstrap-token-manager sync status",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.sync)

	m.prometheus.syncTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootstraptoken_sync_time",
			Help: "kube-bootstrap-token-manager last sync time",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.syncTime)

	m.prometheus.syncCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bootstraptoken_sync_count",
			Help: "kube-bootstrap-token-manager sync count",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.syncCount)

}

func (r *KubeBootstrapTokenManager) initK8s() {
	var err error
	var config *rest.Config

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		// KUBECONFIG
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		// K8S in cluster
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	r.k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

func (m *KubeBootstrapTokenManager) initCloudProvider() {
	var err error

	if m.Opts.CloudProvider.Provider != nil {
		log.Infof("using cloud provider \"%s\"", *m.Opts.CloudProvider.Provider)
		switch *m.Opts.CloudProvider.Provider {
		case "azure":
			if m.Opts.CloudProvider.Config != nil {
				err = os.Setenv("AZURE_AUTH_LOCATION", *m.Opts.CloudProvider.Config)
				if err != nil {
					log.Panic(err)
				}
			}

			// environment
			if m.Opts.CloudProvider.Config != nil {
				m.cloud.azure.environment, err = azure.EnvironmentFromFile(*m.Opts.CloudProvider.Config)
			} else if m.Opts.CloudProvider.Azure.Environment != nil {
				m.cloud.azure.environment, err = azure.EnvironmentFromName(*m.Opts.CloudProvider.Azure.Environment)
			} else {
				m.cloud.azure.environment, err = azure.EnvironmentFromName("AZUREPUBLICCLOUD")
			}
			if err != nil {
				log.Panic(err)
			}

			// auth
			if m.Opts.CloudProvider.Config != nil {
				m.cloud.azure.authorizer, err = auth.NewAuthorizerFromFile(m.cloud.azure.environment.ResourceIdentifiers.KeyVault)
			} else {
				m.cloud.azure.authorizer, err = auth.NewAuthorizerFromEnvironmentWithResource(m.cloud.azure.environment.ResourceIdentifiers.KeyVault)
			}
			if err != nil {
				log.Panic(err)
			}
		}
	}
}

func (m *KubeBootstrapTokenManager) Start() {
	go func() {
		for {
			if err := m.syncRun(); err == nil {
				m.prometheus.sync.WithLabelValues().Set(1)
				m.prometheus.syncCount.WithLabelValues().Inc()
				m.prometheus.syncTime.WithLabelValues().SetToCurrentTime()
			} else {
				log.Error(err)
				m.prometheus.sync.WithLabelValues().Set(0)
			}
			time.Sleep(m.Opts.Sync.Time)
		}
	}()
}

func (m *KubeBootstrapTokenManager) syncRun() error {
	if token := m.fetchCurrentCloudToken(); token != nil {
		contextLogger := log.WithFields(log.Fields{"token": token.Id()})
		if m.checkTokenRenewal(token) {
			contextLogger.Infof("token is not valid or going to expire, starting renewal of token")
			if err := m.createNewToken(); err != nil {
				return err
			}
		} else {
			contextLogger.Infof("valid cloud token, syncing to cluster")
			// sync token
			if err := m.createOrUpdateToken(token, false); err != nil {
				return err
			}
		}
	} else {
		log.Infof("no cloud token found, creating new one")
		if err := m.createNewToken(); err != nil {
			return err
		}
	}

	return nil
}

func (m *KubeBootstrapTokenManager) createNewToken() error {
	token := newBootstrapToken(
		m.generateTokenId(),
		m.generateTokenSecret(),
	)
	token.SetCreationTime(time.Now())

	if m.Opts.BootstrapToken.Expiration != nil {
		token.SetExpirationTime(time.Now().Add(*m.Opts.BootstrapToken.Expiration))
	}

	if err := m.createOrUpdateToken(token, true); err != nil {
		return err
	}

	return nil
}

func (m *KubeBootstrapTokenManager) createOrUpdateToken(token *bootstrapToken, syncToCloud bool) error {
	contextLogger := log.WithFields(log.Fields{"token": token.Id()})

	resourceName := fmt.Sprintf(m.Opts.BootstrapToken.Name, token.Id())
	resourceNs := m.Opts.BootstrapToken.Namespace

	resource, err := m.k8sClient.CoreV1().Secrets(resourceNs).Get(m.ctx, resourceName, v1.GetOptions{})
	if resource == nil && err != nil {
		return err
	}

	if resource == nil || resource.UID == "" {
		resource = &corev1.Secret{}
		resource.SetName(resourceName)
		resource.SetNamespace(resourceNs)

		contextLogger.Infof("creating new bootstrap token \"%s\" with expiration %s", resourceName, token.ExpirationString())
		resource = m.updateTokenData(resource, token)
		if _, err := m.k8sClient.CoreV1().Secrets(resourceNs).Create(m.ctx, resource, v1.CreateOptions{}); err != nil {
			return err
		}
	} else {
		contextLogger.Infof("updating existing bootstrap token \"%s\" with expiration %s", resourceName, token.ExpirationString())
		resource = m.updateTokenData(resource, token)
		if _, err := m.k8sClient.CoreV1().Secrets(resourceNs).Update(m.ctx, resource, v1.UpdateOptions{}); err != nil {
			return err
		}
	}

	if syncToCloud {
		m.storeCurrentCloudToken(token)
	} else {
		contextLogger.Infof("not syncing token to cloud, not needed")
	}

	m.prometheus.token.WithLabelValues(token.Id()).Set(1)
	if token.GetExpirationTime() != nil {
		m.prometheus.tokenExpiration.WithLabelValues(token.Id()).Set(float64(token.GetExpirationTime().Unix()))
	} else {
		m.prometheus.tokenExpiration.WithLabelValues(token.Id()).Set(0)
	}

	return nil
}

func (m *KubeBootstrapTokenManager) updateTokenData(resource *corev1.Secret, token *bootstrapToken) *corev1.Secret {
	resource.Type = corev1.SecretType(m.Opts.BootstrapToken.Type)

	if resource.Labels == nil {
		resource.Labels = map[string]string{}
	}

	resource.Labels[m.Opts.BootstrapToken.Label] = "true"

	if resource.StringData == nil {
		resource.StringData = map[string]string{}
	}

	resource.StringData["description"] = "desc"
	resource.StringData["token-id"] = token.Id()
	resource.StringData["token-secret"] = token.Secret()
	if token.GetExpirationTime() != nil {
		resource.StringData["expiration"] = token.GetExpirationTime().UTC().Format(time.RFC3339)
	}
	resource.StringData["usage-bootstrap-authentication"] = m.Opts.BootstrapToken.UsageBootstrapAuthentication
	resource.StringData["usage-bootstrap-signing"] = m.Opts.BootstrapToken.UsageBootstrapSigning
	resource.StringData["auth-extra-groups"] = m.Opts.BootstrapToken.AuthExtraGroups
	return resource
}

func (m *KubeBootstrapTokenManager) generateTokenId() string {
	return time.Now().Format("060102")
}

func (m *KubeBootstrapTokenManager) generateTokenSecret() string {
	b := make([]rune, m.Opts.BootstrapToken.TokenLength)
	runes := []rune(m.Opts.BootstrapToken.TokenRunes)
	runeLength := len(runes)
	for i := range b {
		b[i] = runes[rand.Intn(runeLength)]
	}
	return string(b)
}

func (m *KubeBootstrapTokenManager) fetchCurrentCloudToken() (token *bootstrapToken) {
	switch *m.Opts.CloudProvider.Provider {
	case "azure":
		if m.Opts.CloudProvider.Azure.KeyVaultName != nil && m.Opts.CloudProvider.Azure.KeyVaultSecretName != nil {
			vaultName := *m.Opts.CloudProvider.Azure.KeyVaultName
			secretName := *m.Opts.CloudProvider.Azure.KeyVaultSecretName
			vaultUrl := fmt.Sprintf(
				"https://%s.%s",
				vaultName,
				m.cloud.azure.environment.KeyVaultDNSSuffix,
			)

			log.Infof("fetching newest token from Azure KeyVault \"%s\" secret \"%s\"", vaultName, secretName)
			secret, err := m.azureKeyvaultClient().GetSecret(m.ctx, vaultUrl, secretName, "")
			if !secret.IsHTTPStatus(404) && err != nil {
				log.Panic(err)
			}

			if secret.Value != nil {
				token = parseBootstrapTokenFromString(*secret.Value)
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
	}

	if token != nil {
		contextLogger := log.WithFields(log.Fields{"token": token.Id()})
		contextLogger.Infof("found cloud token with id \"%s\" and expiration %s", token.Id(), token.ExpirationString())
	}

	return
}

func (m *KubeBootstrapTokenManager) checkTokenRenewal(token *bootstrapToken) bool {
	if token == nil {
		return true
	}

	// no expiry set
	if token.GetExpirationTime() == nil {
		// expiration is enfoced, so renew token
		if m.Opts.BootstrapToken.Expiration != nil {
			return true
		} else {
			// no expiry in token is ok
			return false
		}
	}

	renewalTime := time.Now().Add(m.Opts.Sync.RecreateBefore)
	if token.GetExpirationTime().Before(renewalTime) {
		return true
	}

	return false
}

func (m *KubeBootstrapTokenManager) storeCurrentCloudToken(token *bootstrapToken) {
	contextLogger := log.WithFields(log.Fields{"token": token.Id()})

	if m.Opts.CloudProvider.Provider == nil {
		return
	}

	switch *m.Opts.CloudProvider.Provider {
	case "azure":
		if m.Opts.CloudProvider.Azure.KeyVaultName != nil && m.Opts.CloudProvider.Azure.KeyVaultSecretName != nil {
			vaultName := *m.Opts.CloudProvider.Azure.KeyVaultName
			secretName := *m.Opts.CloudProvider.Azure.KeyVaultSecretName
			vaultUrl := fmt.Sprintf(
				"https://%s.%s",
				vaultName,
				m.cloud.azure.environment.KeyVaultDNSSuffix,
			)

			contextLogger.Infof("storing token to Azure KeyVault \"%s\" secret \"%s\" with expiration %s", vaultName, secretName, token.ExpirationString())

			secretParameters := keyvault.SecretSetParameters{
				Value: stringPtr(token.FullToken()),
				Tags: map[string]*string{
					"managed-by": stringPtr("kube-bootstrap-token-manager"),
					"token":      stringPtr(token.id),
				},
				ContentType: stringPtr("kube-bootstrap-token"),
				SecretAttributes: &keyvault.SecretAttributes{
					NotBefore: token.GetCreationUnixTime(),
					Expires:   token.GetExpirationUnixTime(),
				},
			}
			_, err := m.azureKeyvaultClient().SetSecret(m.ctx, vaultUrl, secretName, secretParameters)
			if err != nil {
				log.Panic(err)
			}
		}
	}
}

func (m *KubeBootstrapTokenManager) azureKeyvaultClient() *keyvault.BaseClient {
	if m.cloud.azure.keyvaultClient == nil {
		auth, err := auth.NewAuthorizerFromEnvironmentWithResource(m.cloud.azure.environment.ResourceIdentifiers.KeyVault)
		if err != nil {
			log.Panic(err)
		}

		client := keyvault.New()
		client.Authorizer = auth
		m.cloud.azure.keyvaultClient = &client
	}

	return m.cloud.azure.keyvaultClient
}
