package session_service

import (
	"context"
	"errors"
	"time"
)

//goland:noinspection GoSnakeCaseUsage
type SessionService[ID, USER_ID comparable] interface {
	CreateSession(context.Context, Session[ID, USER_ID]) error
	UpdateSession(context.Context, Session[ID, USER_ID]) error
	DeleteAllSessionsExceptThis(context.Context, ID) error
	DeleteSessionBySecret(context.Context, string) (Session[ID, USER_ID], error)
	DeleteSessionByID(context.Context, ID) (Session[ID, USER_ID], error)
	DeleteSessionsByUser(context.Context, USER_ID) error
	GetSessionsByUser(context.Context, USER_ID) ([]Session[ID, USER_ID], error)
	GetLastEnterByUser(context.Context, USER_ID) (time.Time, error)
	GetSessionBySecret(context.Context, string) (Session[ID, USER_ID], error)
	AddUniqueIP(_ context.Context, _ ID, ip string) error
}

//goland:noinspection GoSnakeCaseUsage
type Session[ID, USER_ID comparable] struct {
	ID         ID        `json:"id" bson:"_id"`
	Secret     string    `json:"-"  bson:"secret"`
	UserID     USER_ID   `json:"-"  bson:"user_id"`
	IP         []string  `json:"ip" bson:"ip"`
	LastUsage  time.Time `json:"la" bson:"last_usage"`
	UserAgent  string    `json:"ua" bson:"user_agent"`
	AuthMethod string    `json:"am" bson:"auth_method"`
}

var SessionNotFound = errors.New("session not found")
