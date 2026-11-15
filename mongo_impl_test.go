package session_service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	sess "github.com/Alex41/mongo-session-service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const kindMCP sess.Kind = "mcp"

type session = sess.Session[string, uint]

// MONGO_URI points at a disposable server; each test gets its own database.
func newService(t *testing.T) (sess.SessionService[string, uint], *mongo.Database) {
	t.Helper()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set")
	}

	cli, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := cli.Database(fmt.Sprintf("session_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = cli.Disconnect(ctx)
	})

	s, err := sess.MongoImpl[string, uint](ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	return s, db
}

func create(t *testing.T, s sess.SessionService[string, uint], id string, userID uint, kind sess.Kind) {
	t.Helper()
	err := s.CreateSession(context.Background(), session{ID: id, Secret: "secret-" + id, UserID: userID, Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetSessionBySecretFindsOnlySessionsWithoutKind(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	create(t, s, "plain", 1, "")
	create(t, s, "mcp", 1, kindMCP)

	got, err := s.GetSessionBySecret(ctx, "secret-plain")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "plain" || got.Kind != "" || got.Secret != "" {
		t.Fatalf("got %+v, want plain session without kind and secret", got)
	}

	if _, err := s.GetSessionBySecret(ctx, "secret-mcp"); !errors.Is(err, sess.SessionNotFound) {
		t.Fatalf("a kinded secret passed GetSessionBySecret: %v", err)
	}
}

func TestGetSessionBySecretOfKindMatchesTheKindExactly(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	create(t, s, "plain", 1, "")
	create(t, s, "mcp", 1, kindMCP)

	got, err := s.GetSessionBySecretOfKind(ctx, kindMCP, "secret-mcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "mcp" || got.Kind != kindMCP || got.Secret != "" {
		t.Fatalf("got %+v, want mcp session without secret", got)
	}

	for _, c := range []struct {
		kind   sess.Kind
		secret string
	}{
		{kindMCP, "secret-plain"},
		{"other", "secret-mcp"},
	} {
		if _, err := s.GetSessionBySecretOfKind(ctx, c.kind, c.secret); !errors.Is(err, sess.SessionNotFound) {
			t.Fatalf("kind %q found %s: %v", c.kind, c.secret, err)
		}
	}
}

func TestEmptyKindIsNotStored(t *testing.T) {
	s, db := newService(t)
	create(t, s, "plain", 1, "")

	raw, err := db.Collection("session").FindOne(context.Background(), bson.M{"_id": "plain"}).Raw()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.LookupErr("kind"); err == nil {
		t.Fatal("an empty kind was stored")
	}
}

func TestDocumentWithoutKindFieldReadsAsKindless(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()

	_, err := db.Collection("session").InsertOne(ctx, bson.M{"_id": "legacy", "secret": "secret-legacy", "user_id": 1})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSessionBySecret(ctx, "secret-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "legacy" || got.Kind != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetSessionByID(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	create(t, s, "mcp", 7, kindMCP)

	got, err := s.GetSessionByID(ctx, "mcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != 7 || got.Kind != kindMCP || got.Secret != "" {
		t.Fatalf("got %+v, want user 7, kind mcp, no secret", got)
	}

	if _, err := s.GetSessionByID(ctx, "missing"); !errors.Is(err, sess.SessionNotFound) {
		t.Fatalf("missing id: %v", err)
	}
}

func TestDeleteAllSessionsExceptThisKeepsTheCurrentOne(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	create(t, s, "current", 1, "")
	create(t, s, "other", 1, "")
	create(t, s, "mcp", 1, kindMCP)
	create(t, s, "foreign", 2, "")

	if err := s.DeleteAllSessionsExceptThis(ctx, "current"); err != nil {
		t.Fatal(err)
	}

	assertSessions(t, s, 1, "current")
	assertSessions(t, s, 2, "foreign")
}

func TestDeleteSessionsByUserRemovesEveryKind(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	create(t, s, "plain", 1, "")
	create(t, s, "mcp", 1, kindMCP)
	create(t, s, "foreign", 2, kindMCP)

	if err := s.DeleteSessionsByUser(ctx, 1); err != nil {
		t.Fatal(err)
	}

	assertSessions(t, s, 1)
	assertSessions(t, s, 2, "foreign")
}

func TestGetSessionsByUserReturnsKinds(t *testing.T) {
	s, _ := newService(t)
	create(t, s, "plain", 1, "")
	create(t, s, "mcp", 1, kindMCP)

	got, err := s.GetSessionsByUser(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]sess.Kind{}
	for _, g := range got {
		if g.Secret != "" {
			t.Fatalf("session %s carries its secret", g.ID)
		}
		kinds[g.ID] = g.Kind
	}
	if len(kinds) != 2 || kinds["plain"] != "" || kinds["mcp"] != kindMCP {
		t.Fatalf("got kinds %v", kinds)
	}
}

func assertSessions(t *testing.T, s sess.SessionService[string, uint], userID uint, want ...string) {
	t.Helper()

	got, err := s.GetSessionsByUser(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, g := range got {
		ids[g.ID] = true
	}
	if len(ids) != len(want) {
		t.Fatalf("user %d: got %v, want %v", userID, ids, want)
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("user %d: got %v, want %v", userID, ids, want)
		}
	}
}
