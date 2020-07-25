package manager

import (
	"fmt"
	"github.com/Azure/go-autorest/autorest/date"
	"strings"
	"time"
)

type (
	bootstrapToken struct {
		id             string
		secret         string
		creationTime   *time.Time
		expirationTime *time.Time
	}
)

func (t *bootstrapToken) Id() string {
	return t.id
}

func (t *bootstrapToken) Secret() string {
	return t.secret
}

func (t *bootstrapToken) FullToken() string {
	return fmt.Sprintf("%s.%s", t.id, t.secret)
}

func (t *bootstrapToken) SetExpirationTime(val time.Time) {
	t.expirationTime = &val
}

func (t *bootstrapToken) SetExpirationUnixTime(val date.UnixTime) {
	expirationTime := date.UnixEpoch().Add(val.Duration())
	t.expirationTime = &expirationTime
}

func (t *bootstrapToken) GetExpirationTime() *time.Time {
	return t.expirationTime
}

func (t *bootstrapToken) ExpirationString() (expiration string) {
	expiration = "<not set>"
	if t.expirationTime != nil {
		expiration = fmt.Sprintf(
			"%s (%s)",
			t.expirationTime.Format(time.RFC3339),
			t.expirationTime.Sub(time.Now()),
		)
	}

	return
}

func (t *bootstrapToken) GetExpirationUnixTime() (val *date.UnixTime) {
	if t.expirationTime != nil {
		unixTime := date.NewUnixTimeFromDuration(t.expirationTime.Sub(date.UnixEpoch()))
		val = &unixTime
	}
	return
}

func (t *bootstrapToken) SetCreationTime(val time.Time) {
	t.creationTime = &val
}

func (t *bootstrapToken) SetCreationUnixTime(val date.UnixTime) {
	creationTime := date.UnixEpoch().Add(val.Duration())
	t.creationTime = &creationTime
}

func (t *bootstrapToken) GetCreationTime() *time.Time {
	return t.creationTime
}

func (t *bootstrapToken) GetCreationUnixTime() (val *date.UnixTime) {
	if t.creationTime != nil {
		unixTime := date.NewUnixTimeFromDuration(t.creationTime.Sub(date.UnixEpoch()))
		val = &unixTime
	}
	return
}

func newBootstrapToken(id, secret string) *bootstrapToken {
	token := bootstrapToken{
		id:     id,
		secret: secret,
	}
	return &token
}

func parseBootstrapTokenFromString(value string) *bootstrapToken {
	tokenParts := strings.SplitN(value, ".", 2)
	if len(tokenParts) == 2 {
		token := bootstrapToken{
			id:     tokenParts[0],
			secret: tokenParts[1],
		}
		return &token
	}

	return nil
}
