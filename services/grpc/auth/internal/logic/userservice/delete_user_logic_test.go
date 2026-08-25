package userservicelogic

import (
	"strings"
	"testing"
)

func TestDeletedUserTombstoneFitsColumnsAndIsUnique(t *testing.T) {
	firstUsername, firstEmail, err := deletedUserTombstone()
	if err != nil {
		t.Fatalf("deletedUserTombstone() error = %v", err)
	}
	secondUsername, secondEmail, err := deletedUserTombstone()
	if err != nil {
		t.Fatalf("deletedUserTombstone() second error = %v", err)
	}
	if len(firstUsername) > 32 || len(firstEmail) > 254 {
		t.Fatalf("tombstone exceeds column size: username=%d email=%d", len(firstUsername), len(firstEmail))
	}
	if firstUsername == secondUsername || firstEmail == secondEmail {
		t.Fatal("two deleted-user tombstones unexpectedly match")
	}
	if !strings.HasSuffix(firstEmail, "@invalid.local") {
		t.Fatalf("tombstone email %q does not use invalid.local", firstEmail)
	}
}
