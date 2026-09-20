package httpserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/secret"
)

func TestConnectionCredentialsAreEncrypted(t *testing.T) {
	db := mysql.OpenTestDB(t)
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	bareAccess := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	bare := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Access:      bareAccess,
		Idempotency: idempotency.NewStore(db),
	})
	password := "correct horse battery staple"
	username := "cc-" + uuid.NewString()[:8]
	if _, err := authSvc.CreateUser(context.Background(), username, password); err != nil {
		t.Fatal(err)
	}
	sess := loginSession(t, bare, username, password)
	listed := doJSON(t, bare, http.MethodGet, "/api/v1/portfolios", nil, sess, "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list %d %s", listed.Code, listed.Body.String())
	}
	var listBody struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Items) == 0 {
		t.Fatal("expected a default portfolio")
	}
	portfolioID := listBody.Items[0].ID
	ident := doJSON(t, bare, http.MethodPost, "/api/v1/portfolios/"+portfolioID+"/identities", map[string]any{
		"platformId":  catalog.PlatformBinance.String(),
		"externalRef": "cred-acct",
		"displayName": "Binance",
		"accountMode": "binance_pm",
		"kind":        "main",
	}, sess, "")
	if ident.Code != http.StatusCreated {
		t.Fatalf("identity %d %s", ident.Code, ident.Body.String())
	}
	var identBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ident.Body.Bytes(), &identBody); err != nil {
		t.Fatal(err)
	}

	blocked := doJSON(t, bare, http.MethodPost, "/api/v1/portfolios/"+portfolioID+"/connections", map[string]any{
		"identityId": identBody.ID,
		"label":      "primary",
		"status":     "enabled",
		"credentials": map[string]string{
			"apiKey":    "demo-key",
			"apiSecret": "demo-secret",
		},
	}, sess, "cred-create")
	if blocked.Code != http.StatusBadRequest || !strings.Contains(blocked.Body.String(), "master_key_required") {
		t.Fatalf("missing master key %d %s", blocked.Code, blocked.Body.String())
	}

	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, key)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Access:      accessSvc,
		Idempotency: idempotency.NewStore(db),
	})
	secretValue := "demo-secret-" + uuid.NewString()
	created := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+portfolioID+"/connections", map[string]any{
		"identityId": identBody.ID,
		"label":      "primary",
		"status":     "enabled",
		"credentials": map[string]string{
			"apiKey":    "demo-key",
			"apiSecret": secretValue,
		},
	}, sess, "cred-create-ok")
	if created.Code != http.StatusCreated {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	if !strings.Contains(created.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("create missing no-store")
	}
	if strings.Contains(created.Body.String(), secretValue) || strings.Contains(created.Body.String(), `"apiSecret"`) {
		t.Fatalf("response leaked credentials: %s", created.Body.String())
	}
	var connBody struct {
		ID                    string  `json:"id"`
		CredentialsConfigured bool    `json:"credentialsConfigured"`
		CredentialsUpdatedAt  *string `json:"credentialsUpdatedAt"`
		Status                string  `json:"status"`
		Version               int     `json:"version"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &connBody); err != nil {
		t.Fatal(err)
	}
	if !connBody.CredentialsConfigured || connBody.Status != "enabled" || connBody.CredentialsUpdatedAt == nil {
		t.Fatalf("created connection %+v", connBody)
	}

	got := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+portfolioID+"/connections/"+connBody.ID, nil, sess, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get %d %s", got.Code, got.Body.String())
	}
	if strings.Contains(got.Body.String(), secretValue) {
		t.Fatal("get leaked secret")
	}

	connID, err := uuid.Parse(connBody.ID)
	if err != nil {
		t.Fatal(err)
	}
	portID, err := uuid.Parse(portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	row, err := sqlcgen.New(db).GetConnection(context.Background(), sqlcgen.GetConnectionParams{
		PortfolioID: mustBinUUID(portID),
		ID:          mustBinUUID(connID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(row.CredentialsCiphertext, []byte(secretValue)) {
		t.Fatal("ciphertext contained plaintext secret")
	}
	box, err := secret.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := box.Open(connID, portID, row.CredentialKeyVersion, row.CredentialsCiphertext, row.CredentialsNonce)
	if err != nil {
		t.Fatal(err)
	}
	if opened.APIKey != "demo-key" || opened.APISecret != secretValue {
		t.Fatalf("opened %+v", opened)
	}

	if err := mysql.CheckEncryptedConnections(context.Background(), db, true); err != nil {
		t.Fatal(err)
	}
	if err := mysql.CheckEncryptedConnections(context.Background(), db, false); err != mysql.ErrMasterKeyRequired {
		t.Fatalf("ready without key: %v", err)
	}
}

func mustBinUUID(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return b
}
