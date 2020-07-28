package cloudprovider

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
)

type (
	CloudProvider interface {
		Init(ctx context.Context, opts config.Opts)
		FetchToken() (token *bootstraptoken.BootstrapToken)
		FetchTokens() (token []*bootstraptoken.BootstrapToken)
		StoreToken(token *bootstraptoken.BootstrapToken)
	}
)

func NewCloudProvider(provider string) CloudProvider {
	switch provider {
	case "azure":
		return &CloudProviderAzure{}
	}

	log.Panicf("Cloud provider \"%s\" not available", provider)
	return nil
}
