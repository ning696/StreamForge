package signaling

import (
	"encoding/json"
	"errors"
)

const (
	TypeRoomJoin     = "room.join"
	TypeRoomJoined   = "room.joined"
	TypeRoomSnapshot = "room.snapshot"
	TypeRoomLeave    = "room.leave"
	TypePeerJoined   = "peer.joined"
	TypePeerLeft     = "peer.left"
	TypeChatSend     = "chat.send"
	TypeChatMessage  = "chat.message"
	TypePing         = "ping"
	TypePong         = "pong"
	TypeError        = "error"
)

var ErrInvalidMessage = errors.New("invalid signaling message")

type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	RoomID    string          `json:"roomId"`
	PeerID    string          `json:"peerId,omitempty"`
	UserID    int64           `json:"userId"`
	Username  string          `json:"username"`
	Timestamp int64           `json:"timestamp,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func Decode(data []byte) (Message, error) {
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	if message.Type == "" || len(message.Payload) == 0 {
		return Message{}, ErrInvalidMessage
	}
	return message, nil
}

func Payload(value interface{}) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
