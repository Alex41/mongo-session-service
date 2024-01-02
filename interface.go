package session_service

import (
	"context"
	"errors"
	"time"
)

type SessionService interface {
	CreateSession(Session, context.Context) error
	UpdateSession(Session, context.Context) error
	DeleteAllSessionsExceptThis(string /*secret*/, uint64, context.Context) error
	DeleteSessionBySecret(string, context.Context) (Session, error)
	DeleteSessionByID(string, uint64, context.Context) (Session, error)
	GetSessionsByUser(uint64, context.Context) ([]Session, error)
	GetLastEnterByUser(uint64, context.Context) (time.Time, error)
	GetSessionBySecret(string, context.Context) (Session, error)
	AddUniqueIP(ID uint64, sess, ip string, _ context.Context) error
}

type Session struct {
	ID         string    `json:"id" bson:"_id"`
	Secret     string    `json:"-"  bson:"secret"`
	UserId     uint64    `json:"-"  bson:"user_id"`
	IP         []string  `json:"ip" bson:"ip"`
	LastUsage  time.Time `json:"la" bson:"last_usage"`
	UserAgent  string    `json:"ua" bson:"user_agent"`
	AuthMethod string    `json:"am" bson:"auth_method"`
}

var SessionNotFound = errors.New("session not found")
