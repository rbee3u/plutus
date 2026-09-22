package access

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestArchiveRequiresClearChildrenAndConnections(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	user, err := authSvc.CreateUser(ctx, "ar-"+uuid.NewString()[:8], "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	items, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(items) == 0 {
		t.Fatal(err)
	}
	portfolioID := items[0].ID
	svc := NewService(db, 15*time.Minute, 30*time.Minute, nil)
	ref := "uid-" + uuid.NewString()[:8]
	parent, err := svc.CreateIdentity(ctx, portfolioID, &CreateIdentityInput{
		PlatformID: catalog.PlatformBinance, ExternalRef: ref, DisplayName: "Main", AccountMode: catalog.AccountModeBinancePM, Kind: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIdentity(ctx, portfolioID, &CreateIdentityInput{
		PlatformID: catalog.PlatformBinance, ExternalRef: "sub-" + uuid.NewString()[:8], DisplayName: "Sub", AccountMode: catalog.AccountModeBinancePM, Kind: "subordinate", ParentIdentityID: &parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ArchiveIdentity(ctx, portfolioID, parent.ID, parent.Version); err != ErrArchiveSubordinates {
		t.Fatalf("parent archive %v", err)
	}
	conn, err := svc.CreateConnection(ctx, portfolioID, &CreateConnectionInput{IdentityID: parent.ID, Label: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE connections SET status = 'enabled' WHERE id = ?`, conn.ID[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ArchiveIdentity(ctx, portfolioID, child.ID, child.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ArchiveIdentity(ctx, portfolioID, parent.ID, parent.Version); err != ErrArchiveConnection {
		t.Fatalf("connection archive %v", err)
	}
	if _, err := db.Exec(`UPDATE connections SET status = 'disabled' WHERE id = ?`, conn.ID[:]); err != nil {
		t.Fatal(err)
	}
	archived, err := svc.ArchiveIdentity(ctx, portfolioID, parent.ID, parent.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PatchIdentity(ctx, portfolioID, archived.ID, archived.Version, PatchIdentityInput{DisplayName: strPtr("Renamed")}); err != ErrIdentityArchived {
		t.Fatalf("patch archived %v", err)
	}
	again, err := svc.CreateIdentity(ctx, portfolioID, &CreateIdentityInput{
		PlatformID: catalog.PlatformBinance, ExternalRef: ref, DisplayName: "Main again", AccountMode: catalog.AccountModeBinancePM, Kind: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID == archived.ID {
		t.Fatal("recreated identity reused the archived id")
	}
	listed, err := svc.ListIdentities(ctx, portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed {
		if item.ID == archived.ID {
			t.Fatal("archived identity is listed")
		}
	}
}

func strPtr(value string) *string {
	return &value
}
