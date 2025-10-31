// Package domain contains entity without logic, just meta-data
package domain

type UserID string

type User struct {
	ID       UserID
	Username string
}

// NewUser is a tiny helper to avoid ad-hoc struct literals in adapters.
func NewUser(id UserID, username string) *User {
	return &User{ID: id, Username: username}
}
