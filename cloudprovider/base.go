package cloudprovider

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
)

type (
	CloudProvider interface {
		Init(ctx context.Context, opts config.Opts, logger *zap.SugaredLogger, userAgent string)
		FetchToken() (token *bootstraptoken.BootstrapToken)
		FetchTokens() (token []*bootstraptoken.BootstrapToken)
		StoreToken(token *bootstraptoken.BootstrapToken)
	}
)

func NewCloudProvider(provider string) CloudProvider {
	switch strings.ToLower(provider) {
	case "azure":
		return &CloudProviderAzure{}
	}

	panic(fmt.Sprintf("Cloud provider \"%s\" not available", provider))
}
