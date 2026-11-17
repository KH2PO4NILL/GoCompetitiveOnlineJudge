package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/testsupport"
	"gorm.io/gorm"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSensitiveFieldsNeverSerialize(t *testing.T) {
	data, err := json.Marshal(struct {
		User    Users
		Session UserSessions
	}{User: Users{PasswordHash: "private-password"}, Session: UserSessions{RefreshTokenHash: "private-refresh", BrowserSessionHash: sql.NullString{String: "private-browser", Valid: true}}})
	if err != nil || strings.Contains(string(data), "private-") || strings.Contains(string(data), "password_hash") || strings.Contains(string(data), "refresh_token_hash") {
		t.Fatalf("sensitive JSON serialization: %v", err)
	}
}
func seedUser(t *testing.T, db *gorm.DB) *Users {
	t.Helper()
	u := &Users{Username: "alice", Email: "alice@example.com", PasswordHash: "secret-hash", Role: 1, Status: 1}
	if err := NewUsersModel(db).InsertOne(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}
func TestUserBoundarySoftDeletionAndAtomicRollback(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	u := seedUser(t, db)
	users := NewUsersModel(db)
	sessions := NewUserSessionsModel(db)
	var session UserSessions
	if err := db.Transaction(func(tx *gorm.DB) error {
		session = UserSessions{UserId: u.Id, RefreshTokenHash: strings.Repeat("a", 64), ExpiresAt: time.Now().Add(time.Hour)}
		return NewUserSessionsModel(tx).Create(ctx, &session, u)
	}); err != nil {
		t.Fatal(err)
	}
	found, err := users.FindActiveByID(ctx, u.Id)
	if err != nil || found.PasswordHash != "" {
		t.Fatal("public query loaded credentials", err)
	}
	rows, total, err := users.ListActiveUsers(ctx, 1, 20)
	if err != nil || total != 1 || rows[0].PasswordHash != "" {
		t.Fatal("public list loaded credentials", err)
	}
	if _, ok := any(users).(interface {
		Delete(context.Context, int64) error
	}); ok {
		t.Fatal("physical deletion is exposed")
	}
	if err := db.Exec("CREATE TRIGGER reject_revocation BEFORE UPDATE ON user_sessions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'test failure'").Error; err != nil {
		t.Fatal(err)
	}
	if err := users.SoftDeleteAndAnonymize(ctx, u.Id); err == nil {
		t.Fatal("revocation failure did not abort deletion")
	}
	if _, err := users.FindActiveByID(ctx, u.Id); err != nil {
		t.Fatal("failed deletion partially committed", err)
	}
	db.Exec("DROP TRIGGER reject_revocation")
	if err := users.SoftDeleteAndAnonymize(ctx, u.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := users.FindActiveByID(ctx, u.Id); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted user visible", err)
	}
	if _, total, err := users.ListActiveUsers(ctx, 1, 20); err != nil || total != 0 {
		t.Fatal("deleted user listed", err)
	}
	if auth, err := users.FindForAuthenticationByUsername(ctx, "alice"); err != nil || auth != nil {
		t.Fatal("deleted user authenticated", err)
	}
	var saved Users
	db.Take(&saved, u.Id)
	if !saved.DeletedAt.Valid || saved.Username == u.Username || saved.Email == u.Email || saved.PasswordHash != "!deleted" || saved.SchoolId.Valid {
		t.Fatal("incomplete anonymization")
	}
	state, err := sessions.AccessState(ctx, session.Id, u.Id)
	if !errors.Is(err, ErrNotFound) || state != nil {
		t.Fatal("deleted user session valid", err)
	}
	if err := users.SoftDeleteAndAnonymize(ctx, u.Id); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing delete not distinguishable", err)
	}
	db.Take(&session, session.Id)
	if !session.RevokedAt.Valid {
		t.Fatal("session not revoked")
	}
}
func TestSchoolReferenceAndExplicitClearing(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	u := seedUser(t, db)
	users := NewUsersModel(db)
	schools := NewSchoolsModel(db)
	school := &Schools{Name: "School", ShortName: sql.NullString{String: "S", Valid: true}, Country: sql.NullString{String: "CN", Valid: true}, Status: 1}
	if err := schools.InsertOne(ctx, school); err != nil {
		t.Fatal(err)
	}
	if err := users.UpdateProfile(ctx, u.Id, UserProfilePatch{SchoolID: &school.Id}); err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	if err := users.UpdateProfile(ctx, u.Id, UserProfilePatch{SchoolID: &zero}); err != nil {
		t.Fatal(err)
	}
	found, _ := users.FindActiveByID(ctx, u.Id)
	if found.SchoolId.Valid {
		t.Fatal("school not cleared")
	}
	empty := sql.NullString{}
	if err := schools.UpdateDetails(ctx, school.Id, SchoolPatch{ShortName: &empty, Country: &empty, Province: &empty, City: &empty}); err != nil {
		t.Fatal(err)
	}
	saved, _ := schools.FindOne(ctx, school.Id)
	if saved.ShortName.Valid || saved.Country.Valid {
		t.Fatal("nullable fields not cleared")
	}
	if err := users.UpdateProfile(ctx, u.Id, UserProfilePatch{SchoolID: &school.Id}); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&Schools{}, school.Id).Error; err != nil {
		t.Fatal(err)
	}
	found, _ = users.FindActiveByID(ctx, u.Id)
	if found.SchoolId.Valid {
		t.Fatal("school deletion left orphan reference")
	}
	invalid := int64(123456)
	if err := users.UpdateProfile(ctx, u.Id, UserProfilePatch{SchoolID: &invalid}); !errors.Is(err, ErrNotFound) {
		t.Fatal("invalid school accepted", err)
	}
	if err := users.UpdateProfile(ctx, invalid, UserProfilePatch{SchoolID: &zero}); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing user update succeeded", err)
	}
	if err := schools.UpdateDetails(ctx, invalid, SchoolPatch{ShortName: &empty}); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing school update succeeded", err)
	}
	if err := schools.Disable(ctx, invalid); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing school disable succeeded", err)
	}
	duplicate := *u
	duplicate.Id = 0
	if err := users.InsertOne(ctx, &duplicate); !errors.Is(err, ErrDuplicate) {
		t.Fatal("duplicate not distinguishable", err)
	}
}
func TestSessionConcurrentRotationAndStaleLogin(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	u := seedUser(t, db)
	now := time.Now().UTC()
	old := strings.Repeat("a", 64)
	session := &UserSessions{UserId: u.Id, RefreshTokenHash: old, ExpiresAt: now.Add(time.Hour)}
	if err := db.Transaction(func(tx *gorm.DB) error { return NewUserSessionsModel(tx).Create(ctx, session, u) }); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				repo := NewUserSessionsModel(tx)
				locked, _, err := repo.LockRefresh(ctx, old, now)
				if err != nil {
					return err
				}
				return repo.Rotate(ctx, locked.Id, old, strings.Repeat(string(rune('b'+i)), 64), now, now.Add(time.Hour))
			})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrConflict) {
				t.Errorf("rotation: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("single-use rotation succeeded %d times", successes.Load())
	}
	db.Model(&Users{}).Where("id = ?", u.Id).Update("password_hash", "changed")
	stale := &UserSessions{UserId: u.Id, RefreshTokenHash: strings.Repeat("z", 64), ExpiresAt: now.Add(time.Hour)}
	err := db.Transaction(func(tx *gorm.DB) error { return NewUserSessionsModel(tx).Create(ctx, stale, u) })
	if !errors.Is(err, ErrConflict) {
		t.Fatal("pre-reset login created a post-reset session", err)
	}
	if err := NewUserSessionsModel(db).RevokeAll(ctx, u.Id, 0, "test", now); err != nil {
		t.Fatal(err)
	}
	if err := NewUserSessionsModel(db).Rotate(ctx, session.Id, old, strings.Repeat("y", 64), now, now.Add(time.Hour)); !errors.Is(err, ErrConflict) {
		t.Fatal("revoked session rotated", err)
	}
	if err := NewUserSessionsModel(db).Cleanup(ctx, now.Add(2*time.Hour), now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&UserSessions{}).Count(&count)
	if count != 0 {
		t.Fatal("cleanup left expired sessions")
	}
}

func TestBrowserCredentialSurvivesLostRotationResponsesWithoutWeakeningAPITokens(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	u := seedUser(t, db)
	now := time.Now().UTC()
	apiHash := strings.Repeat("a", 64)
	browserHash := strings.Repeat("b", 64)
	session := &UserSessions{UserId: u.Id, RefreshTokenHash: apiHash, BrowserSessionHash: sql.NullString{String: browserHash, Valid: true}, ExpiresAt: now.Add(time.Hour)}
	if err := db.Transaction(func(tx *gorm.DB) error { return NewUserSessionsModel(tx).Create(ctx, session, u) }); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				repo := NewUserSessionsModel(tx)
				locked, _, err := repo.LockBrowserSession(ctx, browserHash, now)
				if err != nil {
					return err
				}
				return repo.Rotate(ctx, locked.Id, locked.RefreshTokenHash, strings.Repeat(string(rune('c'+i)), 64), now, now.Add(time.Hour))
			})
			if err != nil {
				t.Error(err)
			} else {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 4 {
		t.Fatal("browser credential invalidated by interrupted/concurrent rotation")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, err := NewUserSessionsModel(tx).LockRefresh(ctx, apiHash, now)
		return err
	}); !errors.Is(err, ErrNotFound) {
		t.Fatal("old API token became reusable", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, err := NewUserSessionsModel(tx).LockBrowserSession(ctx, apiHash, now)
		return err
	}); !errors.Is(err, ErrNotFound) {
		t.Fatal("API token accepted as browser credential", err)
	}
	if err := NewUserSessionsModel(db).RevokeByBrowserHash(ctx, browserHash, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, _, err := NewUserSessionsModel(tx).LockBrowserSession(ctx, browserHash, now)
		return err
	}); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked browser credential remains valid", err)
	}
}
