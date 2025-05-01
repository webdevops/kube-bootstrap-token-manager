package bootstraptoken

import (
	"fmt"
	"strings"
	"time"
)

type (
	BootstrapToken struct {
		id             string
		secret         string
		creationTime   *time.Time
		expirationTime *time.Time

		annotations map[string]string
	}
)

func NewBootstrapToken(id, secret string) *BootstrapToken {
	token := BootstrapToken{
		id:          id,
		secret:      secret,
		annotations: make(map[string]string),
	}

	return &token
}

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

func (t *BootstrapToken) ExpirationTime() *time.Time {
	return t.expirationTime
}

func (t *BootstrapToken) ExpirationString() (expiration string) {
	expiration = "<not set>"
	if t.expirationTime != nil {
		expiration = fmt.Sprintf(
			"%s (%s)",
			t.expirationTime.Format(time.RFC3339),
			time.Until(*t.expirationTime),
		)
	}

	return
}

func (t *BootstrapToken) SetCreationTime(val time.Time) {
	t.creationTime = &val
}

func (t *BootstrapToken) CreationTime() *time.Time {
	return t.creationTime
}

func ParseFromString(value string) *BootstrapToken {
	tokenParts := strings.SplitN(value, ".", 2)
	if len(tokenParts) == 2 {
		return NewBootstrapToken(tokenParts[0], tokenParts[1])
	}

	return nil
}

func (t *BootstrapToken) SetAnnotation(name, value string) {
	t.annotations[name] = value
}

func (t *BootstrapToken) Annotations() map[string]string {
	return t.annotations
}
