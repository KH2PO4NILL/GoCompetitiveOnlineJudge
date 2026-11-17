package userservicelogic

import (
	"context"
	"database/sql"
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/testsupport"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func adminFixture(t *testing.T) (*gorm.DB, *svc.ServiceContext, *model.Users, *model.Users) {
	t.Helper()
	db := testsupport.Database(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	actor := &model.Users{Username: "root", Email: "root@example.com", Role: 3, Status: 1, PasswordHash: string(hash)}
	target := &model.Users{Username: "alice", Email: "alice@example.com", Role: 1, Status: 1, PasswordHash: string(hash)}
	for _, u := range []*model.Users{actor, target} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	session := &model.UserSessions{UserId: target.Id, RefreshTokenHash: strings.Repeat("a", 64), ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	return db, &svc.ServiceContext{DB: db, UsersModel: model.NewUsersModel(db), SchoolsModel: model.NewSchoolsModel(db)}, actor, target
}
func adminContext(actor *model.Users) context.Context {
	return sharedauth.WithPrincipal(context.Background(), sharedauth.Principal{UserID: actor.Id, Role: pb.UserRole(actor.Role), Status: pb.UserStatus(actor.Status)})
}
func TestAdminAccountPartialUpdatesAndRevocation(t *testing.T) {
	db, services, actor, target := adminFixture(t)
	school := model.Schools{Name: "Test School", Status: 1}
	if err := db.Create(&school).Error; err != nil {
		t.Fatal(err)
	}
	logic := NewAdminUpdateUserLogic(adminContext(actor), services)
	resp, err := logic.AdminUpdateUser(&pb.AdminUpdateUserReq{UserId: target.Id, Username: proto.String("renamed"), SchoolId: proto.Int64(school.Id), NewPassword: proto.String("")})
	if err != nil || resp.User.Username != "renamed" || resp.User.Email != target.Email || resp.User.SchoolId != school.Id {
		t.Fatalf("partial update: %v %v", resp, err)
	}
	var saved model.Users
	db.Take(&saved, target.Id)
	if saved.PasswordHash != target.PasswordHash {
		t.Fatal("empty password changed hash")
	}
	var active int64
	db.Model(&model.UserSessions{}).Where("user_id = ? AND revoked_at IS NULL", target.Id).Count(&active)
	if active != 0 {
		t.Fatal("old sessions remain active")
	}
	resp, err = logic.AdminUpdateUser(&pb.AdminUpdateUserReq{UserId: target.Id, SchoolId: proto.Int64(0), Email: proto.String("CHANGED@example.com"), NewPassword: proto.String("new-password")})
	if err != nil || resp.User.SchoolId != 0 || resp.User.Email != "changed@example.com" {
		t.Fatalf("account update: %v %v", resp, err)
	}
	saved = model.Users{}
	db.Take(&saved, target.Id)
	if saved.SchoolId.Valid || bcrypt.CompareHashAndPassword([]byte(saved.PasswordHash), []byte("new-password")) != nil || bcrypt.CompareHashAndPassword([]byte(saved.PasswordHash), []byte("old-password")) == nil {
		t.Fatal("reset password or school clearing failed")
	}
}
func TestAdminAccountPermissionAndValidationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*gorm.DB, *model.Users, *model.Users, *pb.AdminUpdateUserReq)
		want   codes.Code
	}{
		{"ordinary admin", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { db.Model(a).Update("role", 2) }, codes.PermissionDenied},
		{"disabled actor", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { db.Model(a).Update("status", 2) }, codes.PermissionDenied},
		{"self", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.UserId = a.Id }, codes.PermissionDenied},
		{"admin target", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { db.Model(u).Update("role", 2) }, codes.PermissionDenied},
		{"super target", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { db.Model(u).Update("role", 3) }, codes.PermissionDenied},
		{"deleted target", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) {
			db.Model(u).Update("deleted_at", time.Now())
		}, codes.NotFound},
		{"duplicate username", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.Username = proto.String(a.Username) }, codes.AlreadyExists},
		{"duplicate email", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.Email = proto.String(a.Email) }, codes.AlreadyExists},
		{"missing school", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.SchoolId = proto.Int64(999999) }, codes.InvalidArgument},
		{"disabled school", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) {
			s := model.Schools{Name: "Disabled", Status: 2}
			db.Create(&s)
			r.SchoolId = &s.Id
		}, codes.InvalidArgument},
		{"short password", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.NewPassword = proto.String("short") }, codes.InvalidArgument},
		{"long password", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) {
			r.NewPassword = proto.String(strings.Repeat("中", 25))
		}, codes.InvalidArgument},
		{"invalid email", func(db *gorm.DB, a, u *model.Users, r *pb.AdminUpdateUserReq) { r.Email = proto.String("invalid") }, codes.InvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, services, actor, target := adminFixture(t)
			req := &pb.AdminUpdateUserReq{UserId: target.Id, Username: proto.String("new-name")}
			test.change(db, actor, target, req)
			var before model.Users
			db.Take(&before, target.Id)
			_, err := NewAdminUpdateUserLogic(adminContext(actor), services).AdminUpdateUser(req)
			if status.Code(err) != test.want {
				t.Fatalf("got %v want %s", err, test.want)
			}
			var after model.Users
			db.Take(&after, target.Id)
			if before.Username != after.Username || before.Email != after.Email || before.PasswordHash != after.PasswordHash || before.SchoolId != after.SchoolId {
				t.Fatal("failed request partially updated account")
			}
		})
	}
	t.Run("unauthenticated RPC", func(t *testing.T) {
		_, services, _, target := adminFixture(t)
		_, err := NewAdminUpdateUserLogic(context.Background(), services).AdminUpdateUser(&pb.AdminUpdateUserReq{UserId: target.Id, Email: proto.String("new@example.com")})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatal(err)
		}
	})
}
func TestAdminAccountRechecksActorAndTargetInsideTransaction(t *testing.T) {
	for _, change := range []string{"actor", "target"} {
		t.Run(change, func(t *testing.T) {
			db, services, actor, target := adminFixture(t)
			conn, _ := db.DB()
			var changed atomic.Bool
			// Simulate a permission change after the initial authenticated-user read.
			db.Callback().Query().After("gorm:query").Register("test:permission-change", func(tx *gorm.DB) {
				if tx.Statement.Table == "users" && changed.CompareAndSwap(false, true) {
					var err error
					if change == "actor" {
						_, err = conn.Exec("UPDATE users SET role = 2 WHERE id = ?", actor.Id)
					} else {
						_, err = conn.Exec("UPDATE users SET role = 2 WHERE id = ?", target.Id)
					}
					if err != nil {
						t.Error(err)
					}
				}
			})
			_, err := NewAdminUpdateUserLogic(adminContext(actor), services).AdminUpdateUser(&pb.AdminUpdateUserReq{UserId: target.Id, Email: proto.String("new@example.com")})
			if status.Code(err) != codes.PermissionDenied {
				t.Fatal("stale permissions allowed update", err)
			}
			var saved model.Users
			db.Take(&saved, target.Id)
			if saved.Email != target.Email {
				t.Fatal("stale authorization committed")
			}
		})
	}
}
func TestAdminAccountRevocationFailureRollsBack(t *testing.T) {
	db, services, actor, target := adminFixture(t)
	if err := db.Exec("CREATE TRIGGER reject_revocation BEFORE UPDATE ON user_sessions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'test failure'").Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewAdminUpdateUserLogic(adminContext(actor), services).AdminUpdateUser(&pb.AdminUpdateUserReq{UserId: target.Id, Username: proto.String("rollback-name"), NewPassword: proto.String("new-password")})
	if status.Code(err) != codes.Internal {
		t.Fatal("missing transaction failure", err)
	}
	var saved model.Users
	db.Take(&saved, target.Id)
	if saved.Username != target.Username || saved.PasswordHash != target.PasswordHash {
		t.Fatal("credentials changed without session revocation")
	}
	var session model.UserSessions
	db.Where("user_id = ?", target.Id).Take(&session)
	if session.RevokedAt != (sql.NullTime{}) {
		t.Fatal("partial session revocation")
	}
}
