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

	AppendUniqueTokenToSession(_ context.Context, _ ID, service, token string) error
	RemoveTokenFromSession(_ context.Context, _ ID, service, token string) error
	GetAllTokensByUserAndService(_ context.Context, _ USER_ID, service string) ([]AdditionalToken, error)

	// SetAdditionalData writes one AdditionalData key on an existing session.
	// It updates that key alone rather than rewriting the session, so two
	// concurrent writers setting different keys cannot lose each other's work
	// the way a read-modify-write through UpdateSession would.
	SetAdditionalData(_ context.Context, _ ID, key, value string) error
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

	Tokens map[string][]AdditionalToken `json:"-" bson:"tokens"`

	// AdditionalData carries caller-defined facts about the session that do
	// not warrant a field of their own. Unlike Tokens it is a flat key/value
	// map, so it suits things there is exactly one of per key - a timestamp,
	// a flag, an identifier - rather than an accumulating list.
	//
	// It is nil on sessions written before it existed, so read it defensively.
	// omitempty matters: a nil map would otherwise be stored as null, and
	// Mongo refuses to $set a field inside a null - so the first
	// SetAdditionalData on a freshly created session would fail. Omitting it
	// instead lets $set create the object, which also means documents written
	// before this field existed need no migration.
	AdditionalData map[string]string `json:"-" bson:"additional_data,omitempty"`
}

type AdditionalToken struct {
	Value     string    `bson:"value"`
	CreatedAt time.Time `bson:"created_at"`
}

var SessionNotFound = errors.New("session not found")
