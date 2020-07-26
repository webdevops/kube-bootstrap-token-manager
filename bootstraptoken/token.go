package bootstraptoken

import (
	"fmt"
	"github.com/Azure/go-autorest/autorest/date"
	"strings"
	"time"
)

type (
	BootstrapToken struct {
		id             string
		secret         string
		creationTime   *time.Time
		expirationTime *time.Time
	}
)

func (t *BootstrapToken) Id() string {
	return t.id
}

func (t *BootstrapToken) Secret() string {
	return t.secret
}

func (t *BootstrapToken) FullToken() string {
	return fmt.Sprintf("%s.%s", t.id, t.secret)
}

func (t *BootstrapToken) SetExpirationTime(val time.Time) {
	t.expirationTime = &val
}

func (t *BootstrapToken) SetExpirationUnixTime(val date.UnixTime) {
	expirationTime := date.UnixEpoch().Add(val.Duration())
	t.expirationTime = &expirationTime
}

func (t *BootstrapToken) ExpirationTime() *time.Time {
	return t.expirationTime
}

func (t *BootstrapToken) ExpirationString() (expiration string) {
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

func (t *BootstrapToken) ExpirationUnixTime() (val *date.UnixTime) {
	if t.expirationTime != nil {
		unixTime := date.NewUnixTimeFromDuration(t.expirationTime.Sub(date.UnixEpoch()))
		val = &unixTime
	}
	return
}

func (t *BootstrapToken) SetCreationTime(val time.Time) {
	t.creationTime = &val
}

func (t *BootstrapToken) SetCreationUnixTime(val date.UnixTime) {
	creationTime := date.UnixEpoch().Add(val.Duration())
	t.creationTime = &creationTime
}

func (t *BootstrapToken) CreationTime() *time.Time {
	return t.creationTime
}

func (t *BootstrapToken) CreationUnixTime() (val *date.UnixTime) {
	if t.creationTime != nil {
		unixTime := date.NewUnixTimeFromDuration(t.creationTime.Sub(date.UnixEpoch()))
		val = &unixTime
	}
	return
}

func NewBootstrapToken(id, secret string) *BootstrapToken {
	token := BootstrapToken{
		id:     id,
		secret: secret,
	}
	return &token
}

func ParseFromString(value string) *BootstrapToken {
	tokenParts := strings.SplitN(value, ".", 2)
	if len(tokenParts) == 2 {
		token := BootstrapToken{
			id:     tokenParts[0],
			secret: tokenParts[1],
		}
		return &token
	}

	return nil
}
