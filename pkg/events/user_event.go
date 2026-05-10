package events

import (
	"strings"
	"time"

	pkgevents "gitlab.com/lifegoeson-libs/pkg-gokit/kafka/events"
)

const (
	TopicUserCreated = "modami.auth.user.created"
	TopicUserUpdated = "modami.auth.user.updated"
	TopicSocialLogin = "modami.auth.social.login"
)

// GetTopicWithEnv returns the topic name prefixed with the environment (e.g. "dev.user.events").
// If env is empty, the base topic is returned as-is.
func GetTopicWithEnv(env, topic string) string {
	if env == "" {
		return topic
	}
	return strings.ToLower(env) + "." + topic
}

// UserEntityFields carries all columns from Keycloak's user_entity table.
type UserEntityFields struct {
	ID                       string
	Email                    string
	EmailConstraint          string
	EmailVerified            bool
	Enabled                  bool
	FederationLink           *string
	FirstName                string
	LastName                 string
	RealmID                  string
	Username                 string
	CreatedTimestamp         int64
	ServiceAccountClientLink *string
	NotBefore                int
}

type UserCreatedPayload struct {
	pkgevents.BaseEventPayload
	UserID                   string  `json:"userId"`
	Email                    string  `json:"email"`
	EmailConstraint          string  `json:"emailConstraint"`
	EmailVerified            bool    `json:"emailVerified"`
	Enabled                  bool    `json:"enabled"`
	FederationLink           *string `json:"federationLink"`
	FirstName                string  `json:"firstName"`
	LastName                 string  `json:"lastName"`
	RealmID                  string  `json:"realmId"`
	Username                 string  `json:"username"`
	CreatedTimestamp         int64   `json:"createdTimestamp"`
	ServiceAccountClientLink *string `json:"serviceAccountClientLink"`
	NotBefore                int     `json:"notBefore"`
}

func NewUserCreatedPayload(f UserEntityFields) *UserCreatedPayload {
	return &UserCreatedPayload{
		BaseEventPayload: pkgevents.BaseEventPayload{
			Type:      "user.created",
			Timestamp: time.Now(),
		},
		UserID:                   f.ID,
		Email:                    f.Email,
		EmailConstraint:          f.EmailConstraint,
		EmailVerified:            f.EmailVerified,
		Enabled:                  f.Enabled,
		FederationLink:           f.FederationLink,
		FirstName:                f.FirstName,
		LastName:                 f.LastName,
		RealmID:                  f.RealmID,
		Username:                 f.Username,
		CreatedTimestamp:         f.CreatedTimestamp,
		ServiceAccountClientLink: f.ServiceAccountClientLink,
		NotBefore:                f.NotBefore,
	}
}

type UserUpdatedPayload struct {
	pkgevents.BaseEventPayload
	UserID                   string  `json:"userId"`
	Email                    string  `json:"email"`
	EmailConstraint          string  `json:"emailConstraint"`
	EmailVerified            bool    `json:"emailVerified"`
	Enabled                  bool    `json:"enabled"`
	FederationLink           *string `json:"federationLink"`
	FirstName                string  `json:"firstName"`
	LastName                 string  `json:"lastName"`
	RealmID                  string  `json:"realmId"`
	Username                 string  `json:"username"`
	CreatedTimestamp         int64   `json:"createdTimestamp"`
	ServiceAccountClientLink *string `json:"serviceAccountClientLink"`
	NotBefore                int     `json:"notBefore"`
}

func NewUserUpdatedPayload(f UserEntityFields) *UserUpdatedPayload {
	return &UserUpdatedPayload{
		BaseEventPayload: pkgevents.BaseEventPayload{
			Type:      "user.updated",
			Timestamp: time.Now(),
		},
		UserID:                   f.ID,
		Email:                    f.Email,
		EmailConstraint:          f.EmailConstraint,
		EmailVerified:            f.EmailVerified,
		Enabled:                  f.Enabled,
		FederationLink:           f.FederationLink,
		FirstName:                f.FirstName,
		LastName:                 f.LastName,
		RealmID:                  f.RealmID,
		Username:                 f.Username,
		CreatedTimestamp:         f.CreatedTimestamp,
		ServiceAccountClientLink: f.ServiceAccountClientLink,
		NotBefore:                f.NotBefore,
	}
}

type SocialLoginPayload struct {
	pkgevents.BaseEventPayload
	Provider string `json:"provider"`
	Email    string `json:"email"`
}

func NewSocialLoginPayload(provider, email string) *SocialLoginPayload {
	return &SocialLoginPayload{
		BaseEventPayload: pkgevents.BaseEventPayload{
			Type:      "auth.social.login",
			Timestamp: time.Now(),
		},
		Provider: provider,
		Email:    email,
	}
}
