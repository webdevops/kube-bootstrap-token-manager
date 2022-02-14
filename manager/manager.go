package manager

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/cloudprovider"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"math/big"
	"os"
	"text/template"
	"time"
)

type (
	KubeBootstrapTokenManager struct {
		Opts    config.Opts
		Version string

		ctx       context.Context
		k8sClient *kubernetes.Clientset

		prometheus struct {
			token           *prometheus.GaugeVec
			tokenExpiration *prometheus.GaugeVec

			sync      *prometheus.GaugeVec
			syncTime  *prometheus.GaugeVec
			syncCount *prometheus.CounterVec
		}

		bootstrapToken struct {
			idTemplate *template.Template
		}

		cloudProvider cloudprovider.CloudProvider
	}
)

func (m *KubeBootstrapTokenManager) Init() {
	m.ctx = context.Background()
	m.initK8s()
	m.initPrometheus()
	m.initCloudProvider()

	if t, err := template.New("BootstrapTokenId").Parse(m.Opts.BootstrapToken.IdTemplate); err == nil {
		m.bootstrapToken.idTemplate = t
	} else {
		log.Panic(err)
	}
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
	log.Infof("using cloud provider \"%s\"", *m.Opts.CloudProvider.Provider)
	m.cloudProvider = cloudprovider.NewCloudProvider(*m.Opts.CloudProvider.Provider)
	m.cloudProvider.Init(m.ctx, m.Opts)
}

func (m *KubeBootstrapTokenManager) Start() {
	go func() {
		if m.Opts.Sync.Full {
			log.Infof("starting full sync run")
			if err := m.syncRunFull(); err != nil {
				log.Error(err)
			}
		}
		for {
			log.Infof("starting sync run")
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

func (m *KubeBootstrapTokenManager) syncRunFull() error {
	for _, token := range m.cloudProvider.FetchTokens() {
		contextLogger := log.WithFields(log.Fields{"token": token.Id()})
		contextLogger.Infof("found cloud token with id \"%s\" and expiration %s", token.Id(), token.ExpirationString())
		if !m.checkTokenRenewal(token) {
			contextLogger.Infof("valid cloud token, syncing to cluster")
			// sync token
			if err := m.createOrUpdateToken(token, false); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *KubeBootstrapTokenManager) syncRun() error {
	if token := m.cloudProvider.FetchToken(); token != nil {
		contextLogger := log.WithFields(log.Fields{"token": token.Id()})
		contextLogger.Infof("found cloud token with id \"%s\" and expiration %s", token.Id(), token.ExpirationString())
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
	token := bootstraptoken.NewBootstrapToken(
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

// checks if token already exists, updates if needed otherwise creates token
func (m *KubeBootstrapTokenManager) createOrUpdateToken(token *bootstraptoken.BootstrapToken, syncToCloud bool) error {
	contextLogger := log.WithFields(log.Fields{"token": token.Id()})

	resourceName := fmt.Sprintf(m.Opts.BootstrapToken.Name, token.Id())
	resourceNs := m.Opts.BootstrapToken.Namespace

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		switch {
		case errors.IsServerTimeout(err):
			return true
		case errors.IsConflict(err):
			return true
		case errors.IsTimeout(err):
			return true
		}
		return false
	}, func() error {
		resource, err := m.k8sClient.CoreV1().Secrets(resourceNs).Get(m.ctx, resourceName, v1.GetOptions{})
		if err == nil {
			// update
			contextLogger.Infof("updating existing bootstrap token \"%s\" with expiration %s", resourceName, token.ExpirationString())
			resource = m.updateTokenData(resource, token)
			if _, err := m.k8sClient.CoreV1().Secrets(resourceNs).Update(m.ctx, resource, v1.UpdateOptions{}); err != nil {
				return err
			}
		} else if errors.IsNotFound(err) {
			// create
			resource = &corev1.Secret{}
			resource.SetName(resourceName)
			resource.SetNamespace(resourceNs)

			contextLogger.Infof("creating new bootstrap token \"%s\" with expiration %s", resourceName, token.ExpirationString())
			resource = m.updateTokenData(resource, token)
			if _, err := m.k8sClient.CoreV1().Secrets(resourceNs).Create(m.ctx, resource, v1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			// error
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if syncToCloud {
		m.cloudProvider.StoreToken(token)
	} else {
		contextLogger.Debug("not syncing token to cloud, not needed")
	}

	m.prometheus.token.WithLabelValues(token.Id()).Set(1)
	if token.ExpirationTime() != nil {
		m.prometheus.tokenExpiration.WithLabelValues(token.Id()).Set(float64(token.ExpirationTime().Unix()))
	} else {
		m.prometheus.tokenExpiration.WithLabelValues(token.Id()).Set(0)
	}

	return nil
}

// update kubernetes resource bootstrap token information
func (m *KubeBootstrapTokenManager) updateTokenData(resource *corev1.Secret, token *bootstraptoken.BootstrapToken) *corev1.Secret {
	resource.Type = corev1.SecretType(m.Opts.BootstrapToken.Type)

	if resource.Labels == nil {
		resource.Labels = map[string]string{}
	}

	resource.Labels[m.Opts.BootstrapToken.Label] = "true"

	if resource.StringData == nil {
		resource.StringData = map[string]string{}
	}

	resource.StringData["description"] = fmt.Sprintf("Token maintained by kube-bootstrap-token-manager/%s", m.Version)
	resource.StringData["token-id"] = token.Id()
	resource.StringData["token-secret"] = token.Secret()
	if token.ExpirationTime() != nil {
		resource.StringData["expiration"] = token.ExpirationTime().UTC().Format(time.RFC3339)
	}
	resource.StringData["usage-bootstrap-authentication"] = m.Opts.BootstrapToken.UsageBootstrapAuthentication
	resource.StringData["usage-bootstrap-signing"] = m.Opts.BootstrapToken.UsageBootstrapSigning
	resource.StringData["auth-extra-groups"] = m.Opts.BootstrapToken.AuthExtraGroups
	return resource
}

// creates new token id based on configuration
func (m *KubeBootstrapTokenManager) generateTokenId() string {
	templateData := struct {
		Date string
	}{
		Date: time.Now().UTC().Format("060102"),
	}

	idBuf := &bytes.Buffer{}
	if err := m.bootstrapToken.idTemplate.Execute(idBuf, templateData); err != nil {
		log.Panic(err)
	}
	return idBuf.String()
}

// creates new token secret based on configuration
func (m *KubeBootstrapTokenManager) generateTokenSecret() string {
	b := make([]rune, m.Opts.BootstrapToken.TokenLength)
	runes := []rune(m.Opts.BootstrapToken.TokenRunes)
	runeLength := int64(len(runes))
	for i := range b {
		if val, err := rand.Int(rand.Reader, big.NewInt(runeLength)); err == nil {
			b[i] = runes[val.Uint64()]
		} else {
			log.Panic(err)
		}
	}
	return string(b)
}

// checks if token needs renewal or if enforces expiry date
func (m *KubeBootstrapTokenManager) checkTokenRenewal(token *bootstraptoken.BootstrapToken) bool {
	if token == nil {
		return true
	}

	// no expiry set
	if token.ExpirationTime() == nil {
		// expiration is enfoced, so renew token
		if m.Opts.BootstrapToken.Expiration != nil {
			return true
		} else {
			// no expiry in token is ok
			return false
		}
	}

	renewalTime := time.Now().Add(m.Opts.Sync.RecreateBefore)
	return token.ExpirationTime().Before(renewalTime)
}
