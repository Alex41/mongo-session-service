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

//goland:noinspection GoSnakeCaseUsage
type mongoImpl[ID, USER_ID comparable] struct {
	sess *mongo.Collection
	last *mongo.Collection
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) DeleteSessionsByUser(ctx context.Context, userID USER_ID) error {
	filter := m{"user_id": userID}
	_, err := u.sess.DeleteMany(ctx, filter)
	return err
}

type lastEnter struct {
	UserID    uint64    `bson:"_id"`
	LastEnter time.Time `bson:"last_enter"`
}

type m bson.M

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) GetSessionsByUser(ctx context.Context, userId USER_ID) (s []Session[ID, USER_ID], _ error) {
	filter := m{"user_id": userId}

	cur, err := u.sess.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer cur.Close(ctx)

	err = cur.All(ctx, &s)

	return s, err
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) DeleteAllSessionsExceptThis(ctx context.Context, id ID) error {
	session, err := u.getByID(ctx, id)
	if err != nil {
		return err
	}

	filter := m{"user_id": session.UserID, "secret": m{"$ne": session.Secret}}
	_, err = u.sess.DeleteMany(ctx, filter)
	return err
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) GetLastEnterByUser(ctx context.Context, userId USER_ID) (time.Time, error) {
	filter := m{"_id": userId}

	var result lastEnter
	err := u.last.FindOne(ctx, filter).Decode(&result)

	return result.LastEnter, err
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) CreateSession(ctx context.Context, session Session[ID, USER_ID]) error {
	if session.IP == nil {
		session.IP = make([]string, 0)
	}
	_, e1 := u.sess.InsertOne(ctx, session)

	_, e2 := u.last.UpdateOne(ctx,
		m{"_id": session.UserID},
		m{"$set": m{"last_enter": time.Now()}},
		upsert,
	)

	return searchNotNil(e1, e2)
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) UpdateSession(ctx context.Context, session Session[ID, USER_ID]) error {

	filter1 := m{"_id": session.ID}
	filter2 := m{"_id": session.UserID}

	update1 := m{"$set": m{"user_agent": session.UserAgent, "last_usage": session.LastUsage}}
	update2 := m{"$set": m{"last_enter": time.Now()}}

	_, err1 := u.sess.UpdateOne(ctx, filter1, update1)
	_, err2 := u.last.UpdateOne(ctx, filter2, update2)

	return searchNotNil(err1, err2)
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) getByID(ctx context.Context, id ID) (session Session[ID, USER_ID], e error) {
	sessionIDFilter := m{"_id": id}

	e = u.sess.FindOne(ctx, sessionIDFilter).Decode(&session)
	if errors.Is(e, mongo.ErrNoDocuments) {
		e = SessionNotFound
	}

	return
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) AddUniqueIP(ctx context.Context, id ID, ip string) error {
	session, err := u.getByID(ctx, id)
	if err != nil {
		return err
	}

	var (
		filter1 = m{"_id": session.ID}
		filter2 = m{"_id": session.UserID}
		update  = m{"$addToSet": m{"ip": ip}}
	)

	_, e1 := u.sess.UpdateOne(ctx, filter1, update)
	_, e2 := u.last.UpdateOne(ctx, filter2, update)
	return searchNotNil(e1, e2)
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) DeleteSessionBySecret(ctx context.Context, secret string) (s Session[ID, USER_ID], e error) {
	res := u.sess.FindOneAndDelete(ctx, m{"secret": secret})
	if res.Err() != nil {
		return s, res.Err()
	}

	e = res.Decode(&s)
	return
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) DeleteSessionByID(ctx context.Context, id ID) (s Session[ID, USER_ID], e error) {
	filter := m{"_id": id}
	res := u.sess.FindOneAndDelete(ctx, filter)
	if res.Err() != nil {
		return s, res.Err()
	}

	e = res.Decode(&s)
	return
}

//goland:noinspection GoSnakeCaseUsage
func (u *mongoImpl[ID, USER_ID]) GetSessionBySecret(ctx context.Context, secret string) (s Session[ID, USER_ID], e error) {
	e = u.sess.FindOne(ctx, m{"secret": secret}).Decode(&s)
	if errors.Is(e, mongo.ErrNoDocuments) {
		e = SessionNotFound
	}
	return
}

//goland:noinspection GoSnakeCaseUsage
func MongoImpl[ID, USER_ID comparable](db *mongo.Database, c context.Context) (SessionService[ID, USER_ID], error) {
	u := &mongoImpl[ID, USER_ID]{
		sess: db.Collection("session"),
		last: db.Collection("last_enter"),
	}

	_, e1 := u.sess.Indexes().CreateOne(c, mongo.IndexModel{Keys: m{"secret": 1}, Options: unique})
	_, e2 := u.sess.Indexes().CreateOne(c, mongo.IndexModel{Keys: m{"user_id": 1}})

	return u, searchNotNil(e1, e2)
}
