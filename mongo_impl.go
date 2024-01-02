package session_service

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

var (
	unique = options.Index().SetUnique(true)
	upsert = options.Update().SetUpsert(true)
)

type mongoImpl struct {
	sess *mongo.Collection
	last *mongo.Collection
}

type lastEnter struct {
	UserID    uint64    `bson:"_id"`
	LastEnter time.Time `bson:"last_enter"`
}

type m bson.M

func (u *mongoImpl) GetSessionsByUser(userId uint64, c context.Context) (s []Session, _ error) {
	filter := m{"user_id": userId}

	cur, err := u.sess.Find(c, filter)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer cur.Close(c)

	err = cur.All(c, &s)

	return s, err
}
func (u *mongoImpl) DeleteAllSessionsExceptThis(secret string, uID uint64, ctx context.Context) error {
	filter := m{"user_id": uID, "secret": m{"$ne": secret}}
	_, err := u.sess.DeleteMany(ctx, filter)
	return err
}

func (u *mongoImpl) GetLastEnterByUser(userId uint64, ctx context.Context) (time.Time, error) {
	filter := m{"_id": userId}

	var result lastEnter
	err := u.last.FindOne(ctx, filter).Decode(&result)

	return result.LastEnter, err
}

func (u *mongoImpl) CreateSession(session Session, ctx context.Context) error {
	if session.IP == nil {
		session.IP = make([]string, 0)
	}
	_, e1 := u.sess.InsertOne(ctx, session)

	_, e2 := u.last.UpdateOne(ctx,
		m{"_id": session.UserId},
		m{"$set": m{"last_enter": time.Now()}},
		upsert,
	)

	return searchNotNil(e1, e2)
}

func (u *mongoImpl) UpdateSession(session Session, ctx context.Context) error {

	filter1 := m{"_id": session.ID}
	filter2 := m{"_id": session.UserId}

	update1 := m{"$set": m{"user_agent": session.UserAgent, "last_usage": session.LastUsage}}
	update2 := m{"$set": m{"last_enter": time.Now()}}

	_, err1 := u.sess.UpdateOne(ctx, filter1, update1)
	_, err2 := u.last.UpdateOne(ctx, filter2, update2)

	return searchNotNil(err1, err2)
}

func (u *mongoImpl) AddUniqueIP(userID uint64, sessionID string, ip string, ctx context.Context) error {

	var (
		filter1 = m{"_id": sessionID}
		filter2 = m{"_id": userID}
		update  = m{"$addToSet": m{"ip": ip}}
	)

	_, e1 := u.sess.UpdateOne(ctx, filter1, update)
	_, e2 := u.last.UpdateOne(ctx, filter2, update)
	return searchNotNil(e1, e2)
}

func (u *mongoImpl) DeleteSessionBySecret(secret string, ctx context.Context) (s Session, e error) {
	res := u.sess.FindOneAndDelete(ctx, m{"secret": secret})
	if res.Err() != nil {
		return s, res.Err()
	}

	e = res.Decode(&s)
	return
}

func (u *mongoImpl) DeleteSessionByID(id string, uID uint64, ctx context.Context) (s Session, e error) {
	filter := m{"user_id": uID, "_id": id}
	res := u.sess.FindOneAndDelete(ctx, filter)
	if res.Err() != nil {
		return s, res.Err()
	}

	e = res.Decode(&s)
	return
}

func (u *mongoImpl) GetSessionBySecret(secret string, ctx context.Context) (s Session, e error) {
	e = u.sess.FindOne(ctx, m{"secret": secret}).Decode(&s)
	if errors.Is(e, mongo.ErrNoDocuments) {
		e = SessionNotFound
	}
	return
}

func MongoImpl(db *mongo.Database, c context.Context) (SessionService, error) {
	u := &mongoImpl{
		sess: db.Collection("session"),
		last: db.Collection("last_enter"),
	}

	_, e1 := u.sess.Indexes().CreateOne(c, mongo.IndexModel{Keys: m{"secret": 1}, Options: unique})
	_, e2 := u.sess.Indexes().CreateOne(c, mongo.IndexModel{Keys: m{"user_id": 1}})

	return u, searchNotNil(e1, e2)
}
