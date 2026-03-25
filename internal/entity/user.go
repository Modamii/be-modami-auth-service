package entity

type User struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Enabled           bool   `json:"enabled"`
}
