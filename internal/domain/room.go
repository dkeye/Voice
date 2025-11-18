package domain

type (
	RoomName string
	RoomID   string
)

type Room struct {
	ID   RoomID
	Name RoomName
}
