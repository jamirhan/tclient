package tclient

type ChatID int64

type User struct {
	ID int64 `json:"id"`
}

type Chat struct {
	ID ChatID `json:"id"`
}

type MessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type Message struct {
	From            User            `json:"from"`
	Chat            Chat            `json:"chat"`
	Text            string          `json:"text"`
	MessageEntities []MessageEntity `json:"entities"`
	ReplyToMessage  *Message        `json:"reply_to_message"`
}

type sendMessageRequest struct {
	ChatID ChatID
	Text   string
}
