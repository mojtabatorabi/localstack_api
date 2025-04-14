// api/models/message.go

package models

import (
	"time"
)

type Message struct {
	ID          int       `db:"id" json:"id"`
	UserID      int       `db:"user_id" json:"userId"`
	Message     string    `db:"message" json:"message"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	ProcessedAt time.Time `db:"processed_at" json:"processedAt,omitempty"`
}
