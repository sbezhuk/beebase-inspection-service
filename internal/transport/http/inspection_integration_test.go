//go:build integration

package http_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/platform/hiveclient"
	repopostgres "github.com/sbezhuk/beebase-inspection-service/internal/repository/postgres"
	transporthttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http"
	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"

	"github.com/sbezhuk/beebase-common/authmw"
	"github.com/sbezhuk/beebase-common/jwks"
	"github.com/sbezhuk/beebase-common/logger"
)

const testKID = "test-kid"

// fakeHiveService stands in for the real hive-service: it owns exactly
// one hive per bearer token registered via allow, and answers
// GET /api/v1/hives/{id} exactly like the real service would - 200 if the
// presented token's owner owns that hive, 404 otherwise - so this test
// exercises inspection-service's real cross-service HTTP call without
// needing a second full service running.
type fakeHiveService struct {
	mu    sync.Mutex
	owned map[string]uuid.UUID // "Bearer <token>" -> the one hive it owns
}

func newFakeHiveService() *fakeHiveService {
	return &fakeHiveService{owned: map[string]uuid.UUID{}}
}

func (f *fakeHiveService) allow(token string, hiveID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.owned["Bearer "+token] = hiveID
}

func (f *fakeHiveService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	owned, ok := f.owned[r.Header.Get("Authorization")]
	f.mu.Unlock()

	hiveID, err := uuid.Parse(strings.TrimPrefix(r.URL.Path, "/api/v1/hives/"))
	if err != nil || !ok || owned != hiveID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type testStack struct {
	server *httptest.Server
	hive   *fakeHiveService
	priv   ed25519.PrivateKey
}

// newTestStack wires a full router against a real PostgreSQL database
// (every write scoped to a transaction rolled back at the end of the
// test), a real JWKS server, and a fake hive-service - exactly mirroring
// how inspection-service verifies tokens and hive ownership in
// production, just with throwaway stand-ins instead of the real services.
func newTestStack(t *testing.T) *testStack {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping HTTP inspection integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jwksHandler, err := jwks.NewHandler(pub, testKID)
	if err != nil {
		t.Fatalf("jwks.NewHandler: %v", err)
	}
	jwksServer := httptest.NewServer(jwksHandler)
	t.Cleanup(jwksServer.Close)

	verifier, err := authmw.NewVerifierFromJWKSURL(context.Background(), jwksServer.URL)
	if err != nil {
		t.Fatalf("NewVerifierFromJWKSURL: %v", err)
	}

	hive := newFakeHiveService()
	hiveServer := httptest.NewServer(hive)
	t.Cleanup(hiveServer.Close)

	inspectionRepo := repopostgres.NewInspectionRepository(tx)
	hiveVerifier := hiveclient.New(hiveServer.URL)
	inspectionService := appinspection.NewService(inspectionRepo, hiveVerifier)
	log := logger.New("development", "error")
	handler := inspectionhttp.NewHandler(inspectionService, log)

	router := transporthttp.NewRouter(log, pool, handler, verifier)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testStack{server: srv, hive: hive, priv: priv}
}

func (s *testStack) tokenFor(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = testKID

	signed, err := token.SignedString(s.priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func (s *testStack) request(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, s.server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

const testInspectedAt = "2026-03-15T09:00:00Z"

func TestInspectionFlow_CreateGetListUpdateDelete(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	// Create
	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      hiveID.String(),
		"inspected_at": testInspectedAt,
		"notes":        "queen seen, brood pattern good",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created inspectionhttp.Response
	decodeJSON(t, resp, &created)
	if created.HiveID != hiveID {
		t.Fatalf("create: hive_id = %s, want %s", created.HiveID, hiveID)
	}

	// Get
	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+created.ID.String(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// List for the hive
	resp = stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/inspections", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []inspectionhttp.Response
	decodeJSON(t, resp, &list)
	if len(list) != 1 {
		t.Fatalf("list: got %d inspections, want 1", len(list))
	}

	// Update
	resp = stack.request(t, http.MethodPut, "/api/v1/inspections/"+created.ID.String(), token, map[string]string{
		"inspected_at": "2026-03-16T09:00:00Z",
		"notes":        "re-inspected: all good",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated inspectionhttp.Response
	decodeJSON(t, resp, &updated)
	if updated.Notes != "re-inspected: all good" {
		t.Fatalf("update: notes = %q, want %q", updated.Notes, "re-inspected: all good")
	}
	if updated.HiveID != hiveID {
		t.Fatalf("update: hive_id changed to %s, want unchanged %s", updated.HiveID, hiveID)
	}

	// Delete
	resp = stack.request(t, http.MethodDelete, "/api/v1/inspections/"+created.ID.String(), token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	// Get after delete: gone
	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+created.ID.String(), token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestInspectionFlow_CreateRejectedWhenHiveNotOwned is the end-to-end
// proof that an inspection can't be created under a hive the caller
// doesn't own, verified against a real cross-service HTTP call.
func TestInspectionFlow_CreateRejectedWhenHiveNotOwned(t *testing.T) {
	stack := newTestStack(t)
	token := stack.tokenFor(t, uuid.New())
	someoneElsesHive := uuid.New()
	// Deliberately not calling stack.hive.allow for this token/hive pair.

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      someoneElsesHive.String(),
		"inspected_at": testInspectedAt,
		"notes":        "snooping",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("create under unowned hive: status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	errBody, _ := body["error"].(map[string]any)
	if errBody["code"] != "hive_not_found" {
		t.Fatalf("error code = %v, want hive_not_found", errBody["code"])
	}
}

// TestInspectionFlow_CannotAccessAnotherUsersInspection is the end-to-end
// proof of this module's central requirement, exercised over real HTTP
// with real JWT verification for two different users.
func TestInspectionFlow_CannotAccessAnotherUsersInspection(t *testing.T) {
	stack := newTestStack(t)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()
	ownerToken := stack.tokenFor(t, owner)
	otherToken := stack.tokenFor(t, other)
	stack.hive.allow(ownerToken, hiveID)

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", ownerToken, map[string]string{
		"hive_id":      hiveID.String(),
		"inspected_at": testInspectedAt,
		"notes":        "owner's inspection",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created inspectionhttp.Response
	decodeJSON(t, resp, &created)

	cases := []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]string{"inspected_at": testInspectedAt, "notes": "hijacked"}},
		{http.MethodDelete, nil},
	}
	for _, tc := range cases {
		resp := stack.request(t, tc.method, "/api/v1/inspections/"+created.ID.String(), otherToken, tc.body)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s as a different user: status = %d, want %d", tc.method, resp.StatusCode, http.StatusNotFound)
		}
	}

	// The other user's list for the same hive must come back empty, not
	// an error - they simply have no inspections there.
	resp = stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/inspections", otherToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []inspectionhttp.Response
	decodeJSON(t, resp, &list)
	if len(list) != 0 {
		t.Fatalf("other user's list = %v, want empty", list)
	}

	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+created.ID.String(), ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner get after other user's attempts: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var stillOwners inspectionhttp.Response
	decodeJSON(t, resp, &stillOwners)
	if stillOwners.Notes != "owner's inspection" {
		t.Fatalf("notes = %q after other user's attempts, want unchanged", stillOwners.Notes)
	}
}

func TestInspectionFlow_WithoutTokenIsUnauthorized(t *testing.T) {
	stack := newTestStack(t)

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+uuid.New().String()+"/inspections", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("list without token: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestInspectionFlow_ValidationErrors(t *testing.T) {
	stack := newTestStack(t)
	token := stack.tokenFor(t, uuid.New())

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      uuid.New().String(),
		"inspected_at": testInspectedAt,
		"notes":        "",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with empty notes: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      "not-a-uuid",
		"inspected_at": testInspectedAt,
		"notes":        "ok",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with malformed hive_id: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      uuid.New().String(),
		"inspected_at": "not-a-date",
		"notes":        "ok",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with malformed inspected_at: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/not-a-uuid", token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("get with malformed inspection id: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodGet, "/api/v1/hives/not-a-uuid/inspections", token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("list with malformed hive id: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
