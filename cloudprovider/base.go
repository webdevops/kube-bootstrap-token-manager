package cloudprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/webdevops/go-common/log/slogger"

	"github.com/webdevops/kube-bootstrap-token-manager/bootstraptoken"
	"github.com/webdevops/kube-bootstrap-token-manager/config"
)

type (
	CloudProvider interface {
		Init(ctx context.Context, opts config.Opts, logger *slogger.Logger, userAgent string)
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

	panic(fmt.Sprintf("cloud provider \"%s\" not available", provider))
}
