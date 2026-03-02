package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	PublicKey string    `json:"-"` // Never expose in JSON
	CreatedAt time.Time `json:"created_at"`
}

type Challenge struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	Username    string    `json:"username"`
	Challenge   string    `json:"challenge"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}
