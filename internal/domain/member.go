package domain

// Member represents user's participation meta for a room.
// No transport or lifecycle logic here.
type Member struct {
	User *User
	Mute bool
	// role, mute, anon, etc. could go here later
}

// NewMember avoids raw literals in adapters and keeps construction obvious.
func NewMember(user *User) *Member {
	return &Member{User: user}
}
