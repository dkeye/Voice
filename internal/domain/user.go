// Package domain contains entity without logic, just meta-data
package domain

import (
	"errors"

	"github.com/google/uuid"
)

const (
	MaxUserIDLen   = 36
	MaxUsernameLen = 36
)

var (
	ErrUsernameTooLong = errors.New("username too long")
	ErrUsernameEmpty   = errors.New("username empty")
)

type UserID string

type User struct {
	ID       UserID `json:"id"`
	Username string `json:"username"`
}

// NewUser is a tiny helper to avoid ad-hoc struct literals in adapters.
func NewUser(username string) (*User, error) {
	if len(username) == 0 {
		return nil, ErrUsernameEmpty
	}
	if len(username) > MaxUsernameLen {
		return nil, ErrUsernameTooLong
	}
	id := UserID(uuid.NewString())
	return &User{ID: id, Username: username}, nil
}

func (u *User) SetUsername(username string) error {
	if len(username) == 0 {
		return ErrUsernameEmpty
	}
	if len(username) > MaxUsernameLen {
		return ErrUsernameTooLong
	}
	u.Username = username
	return nil
}
