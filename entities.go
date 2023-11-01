package tclient

type ChatID int64

type User struct {
	ID int64 `json:"id"`
}

type Chat struct {
	ID ChatID `json:"id"`
}

type Message struct {
	From User `json:"from"`
	Chat Chat `json:"chat"`
}
