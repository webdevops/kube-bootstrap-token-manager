package config

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Trace   bool `           long:"trace"        env:"TRACE"    description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		BootstrapToken struct {
			IdTemplate                   string         `long:"bootstraptoken.id-template"                     env:"BOOTSTRAPTOKEN_ID_TEMPLATE"                        description:"Template for token ID for bootstrap tokens" default:"{{.Date}}"`
			Name                         string         `long:"bootstraptoken.name"                            env:"BOOTSTRAPTOKEN_NAME"                               description:"Name for bootstrap tokens" default:"bootstrap-token-%s"`
			Label                        string         `long:"bootstraptoken.label"                           env:"BOOTSTRAPTOKEN_LABEL"                              description:"Label for bootstrap tokens" default:"webdevops.kubernetes.io/bootstraptoken-managed"`
			Namespace                    string         `long:"bootstraptoken.namespace"                       env:"BOOTSTRAPTOKEN_NAMESPACE"                          description:"Namespace for bootstrap tokens" default:"kube-system"`
			Type                         string         `long:"bootstraptoken.type"                            env:"BOOTSTRAPTOKEN_TYPE"                               description:"Type for bootstrap tokens" default:"bootstrap.kubernetes.io/token"`
			UsageBootstrapAuthentication string         `long:"bootstraptoken.usage-bootstrap-authentication"  env:"BOOTSTRAPTOKEN_USAGE_BOOTSTRAP_AUTHENTICATION"     description:"Usage bootstrap authentication for bootstrap tokens" default:"true"`
			UsageBootstrapSigning        string         `long:"bootstraptoken.usage-bootstrap-signing"         env:"BOOTSTRAPTOKEN_USAGE_BOOTSTRAP_SIGNING"            description:"usage bootstrap signing for bootstrap tokens" default:"true"`
			AuthExtraGroups              string         `long:"bootstraptoken.auth-extra-groups"               env:"BOOTSTRAPTOKEN_AUTH_EXTRA_GROUPS"                  description:"Auth extra groups for bootstrap tokens" default:"system:bootstrappers:worker,system:bootstrappers:ingress"`
			Expiration                   *time.Duration `long:"bootstraptoken.expiration"                      env:"BOOTSTRAPTOKEN_EXPIRATION"                         description:"Expiration (time.Duration) for bootstrap tokens" default:"8760h"`
			TokenLength                  uint           `long:"bootstraptoken.token-length"                    env:"BOOTSTRAPTOKEN_TOKEN_LENGTH"                       description:"Length of the random token string for bootstrap tokens" default:"16"`
			TokenRunes                   string         `long:"bootstraptoken.token-runes"                     env:"BOOTSTRAPTOKEN_TOKEN_RUNES"                        description:"Runes which should be used for the random token string for bootstrap tokens" default:"abcdefghijklmnopqrstuvwxyz0123456789"`
		}

		Sync struct {
			Time           time.Duration `long:"sync.time"               env:"SYNC_TIME"                 description:"Sync time (time.Duration)" default:"1h"`
			RecreateBefore time.Duration `long:"sync.recreate-before"    env:"SYNC_RECREATE_BEFORE"      description:"Time duration (time.Duration) when token should be recreated" default:"2190h"`
			Full           bool          `long:"sync.full"               env:"SYNC_FULL"                 description:"Sync also previous tokens (full sync)"`
		}

		CloudProvider struct {
			Provider *string `long:"cloud-provider"  env:"CLOUD_PROVIDER"       description:"Cloud provider" choice:"azure" required:"true"`
			Config   *string `long:"cloud-config"    env:"CLOUD_CONFIG"         description:"Cloud provider configuration path"`

			Azure struct {
				Environment        *string `long:"azure-environment"            env:"AZURE_ENVIRONMENT"                description:"Azure environment name"`
				KeyVaultName       *string `long:"azure.keyvault-name"          env:"AZURE_KEYVAULT_NAME"              description:"Name of Keyvault to sync token"`
				KeyVaultSecretName *string `long:"azure.keyvault-secret-name"   env:"AZURE_KEYVAULT_SECRET_NAME"       description:"Name of Keyvault secret to sync token" default:"kube-bootstrap-token"`
			}
		}

		// general options
		DryRun     bool   `long:"dry-run"  env:"DRY_RUN"       description:"Dry run (do not apply to nodes)"`
		ServerBind string `long:"bind"     env:"SERVER_BIND"   description:"Server address"     default:":8080"`
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}
