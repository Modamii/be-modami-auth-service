package events

import (
	"time"

	pkgevents "gitlab.com/lifegoeson-libs/pkg-gokit/kafka/events"
)

const (
	TopicUserCreated  = "modami.auth.user.created"
	TopicUserUpdated  = "modami.auth.user.updated"
	TopicSocialLogin  = "modami.auth.social.login"
)

type UserCreatedPayload struct {
	pkgevents.BaseEventPayload
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func NewUserCreatedPayload(userID, email, username, firstName, lastName string) *UserCreatedPayload {
	return &UserCreatedPayload{
		BaseEventPayload: pkgevents.BaseEventPayload{
			Type:      "user.created",
			Timestamp: time.Now(),
		},
		UserID:    userID,
		Email:     email,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
	}
}

type UserUpdatedPayload struct {
	pkgevents.BaseEventPayload
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func NewUserUpdatedPayload(userID, email, firstName, lastName string) *UserUpdatedPayload {
	return &UserUpdatedPayload{
		BaseEventPayload: pkgevents.BaseEventPayload{
			Type:      "user.updated",
			Timestamp: time.Now(),
		},
		UserID:    userID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
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
