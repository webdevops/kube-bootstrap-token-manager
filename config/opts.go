package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Level  string `long:"log.level"    env:"LOG_LEVEL"   description:"Log level" choice:"trace" choice:"debug" choice:"info" choice:"warning" choice:"error" default:"info"`                          // nolint:staticcheck // multiple choices are ok
			Format string `long:"log.format"   env:"LOG_FORMAT"  description:"Log format" choice:"logfmt" choice:"json" default:"logfmt"`                                                                     // nolint:staticcheck // multiple choices are ok
			Source string `long:"log.source"   env:"LOG_SOURCE"  description:"Show source for every log message (useful for debugging and bug reports)" choice:"" choice:"short" choice:"file" choice:"full"` // nolint:staticcheck // multiple choices are ok
			Color  string `long:"log.color"    env:"LOG_COLOR"   description:"Enable color for logs" choice:"" choice:"auto" choice:"yes" choice:"no"`                                                        // nolint:staticcheck // multiple choices are ok
			Time   bool   `long:"log.time"     env:"LOG_TIME"    description:"Show log time"`
		}

		BootstrapToken struct {
			IdTemplate                   string         `long:"bootstraptoken.id-template"                     env:"BOOTSTRAPTOKEN_ID_TEMPLATE"                        description:"Template for token ID for bootstrap tokens" default:"{{.Date}}"`
			Name                         string         `long:"bootstraptoken.name"                            env:"BOOTSTRAPTOKEN_NAME"                               description:"Name for bootstrap tokens" default:"bootstrap-token-%s"`
			Label                        string         `long:"bootstraptoken.label"                           env:"BOOTSTRAPTOKEN_LABEL"                              description:"Label for bootstrap tokens" default:"bootstraptoken.webdevops.io/managed"`
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

			Azure struct {
				KeyVaultUrl        *string `long:"azure.keyvault.url"       env:"AZURE_KEYVAULT_URL"          description:"URL of Keyvault to sync token"`
				KeyVaultSecretName *string `long:"azure.keyvault.secret"    env:"AZURE_KEYVAULT_SECRET"       description:"Name of Keyvault secret to sync token" default:"kube-bootstrap-token"`
			}
		}

		// general options
		DryRun bool `long:"dry-run"  env:"DRY_RUN"       description:"Dry run (do not apply to nodes)"`

		// general options
		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`
		}
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
