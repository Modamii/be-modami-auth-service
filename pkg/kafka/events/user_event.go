package events

import "time"

// UserCreatedPayload is the payload for the user.created event.
type UserCreatedPayload struct {
	BaseEventPayload
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func NewUserCreatedPayload(userID, email, username, firstName, lastName string) *UserCreatedPayload {
	return &UserCreatedPayload{
		BaseEventPayload: BaseEventPayload{
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

// UserUpdatedPayload is the payload for the user.updated event.
type UserUpdatedPayload struct {
	BaseEventPayload
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func NewUserUpdatedPayload(userID, email, firstName, lastName string) *UserUpdatedPayload {
	return &UserUpdatedPayload{
		BaseEventPayload: BaseEventPayload{
			Type:      "user.updated",
			Timestamp: time.Now(),
		},
		UserID:    userID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}
}
