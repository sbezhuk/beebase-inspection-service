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
	"github.com/sbezhuk/beebase-inspection-service/internal/platform/mediaclient"
	repopostgres "github.com/sbezhuk/beebase-inspection-service/internal/repository/postgres"
	transporthttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http"
	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"

	"github.com/sbezhuk/beebase-common/authmw"
	"github.com/sbezhuk/beebase-common/jwks"
	"github.com/sbezhuk/beebase-common/logger"
	"github.com/sbezhuk/beebase-common/pagination"
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

// fakeMediaService stands in for media-service's GET /api/v1/media?ids=
// and DELETE /api/v1/media?ids= endpoints, which inspection-service now
// calls to verify image ownership on create/update and to hard-delete an
// inspection's own files when its hive is cascade-deleted: it answers GET
// from an in-memory set of media ids a test can seed as belonging to the
// caller via own(), and 204 to DELETE. It records every request it
// received, so tests can assert the cascade actually reached it, without
// running a second full service.
type fakeMediaService struct {
	mu       sync.Mutex
	received []*http.Request
	ownedIDs map[uuid.UUID]bool
}

func newFakeMediaService() *fakeMediaService {
	return &fakeMediaService{ownedIDs: map[uuid.UUID]bool{}}
}

// own registers each of ids as belonging to the caller, so this fake's
// GET /api/v1/media?ids= endpoint returns it.
func (f *fakeMediaService) own(ids ...uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		f.ownedIDs[id] = true
	}
}

func (f *fakeMediaService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.received = append(f.received, r.Clone(r.Context()))
	f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/media":
		f.serveList(w, r)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// serveList answers GET /api/v1/media?ids=&ids=...: returns every
// requested id this fake was told is own()ed by the caller, silently
// omitting unknown/foreign ones - mirroring media-service's real
// behavior closely enough for the ownership verification to be exercised
// against it.
func (f *fakeMediaService) serveList(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	items := []map[string]any{}
	for _, raw := range r.URL.Query()["ids"] {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		if f.ownedIDs[id] {
			items = append(items, map[string]any{"id": id})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// calledWithQueryValue reports whether any received request's repeated
// query param key (e.g. ?ids=&ids=...) includes value among its values.
func (f *fakeMediaService) calledWithQueryValue(key, value string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.received {
		for _, v := range r.URL.Query()[key] {
			if v == value {
				return true
			}
		}
	}
	return false
}

// calledDeleteWithQueryValue reports whether any received DELETE
// request's repeated query param key includes value among its values.
func (f *fakeMediaService) calledDeleteWithQueryValue(key, value string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.received {
		if r.Method != http.MethodDelete {
			continue
		}
		for _, v := range r.URL.Query()[key] {
			if v == value {
				return true
			}
		}
	}
	return false
}

type testStack struct {
	server *httptest.Server
	hive   *fakeHiveService
	media  *fakeMediaService
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

	media := newFakeMediaService()
	mediaServer := httptest.NewServer(media)
	t.Cleanup(mediaServer.Close)

	inspectionRepo := repopostgres.NewInspectionRepository(tx)
	hiveVerifier := hiveclient.New(hiveServer.URL)
	mediaClient := mediaclient.New(mediaServer.URL)
	inspectionService := appinspection.NewService(inspectionRepo, hiveVerifier, mediaClient)
	log := logger.New("development", "error")
	handler := inspectionhttp.NewHandler(inspectionService, log, "http://localhost:8080")

	router := transporthttp.NewRouter(log, pool, handler, verifier)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testStack{server: srv, hive: hive, media: media, priv: priv}
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
		"type":         "QUEEN",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created inspectionhttp.Response
	decodeJSON(t, resp, &created)
	if created.HiveID != hiveID {
		t.Fatalf("create: hive_id = %s, want %s", created.HiveID, hiveID)
	}
	if created.Type != "QUEEN" || created.TypeLabel != "Queen" {
		t.Fatalf("create: type = %q, type_label = %q, want %q, %q", created.Type, created.TypeLabel, "QUEEN", "Queen")
	}

	// Get - the type set at creation must be correctly restored when
	// reopening the inspection for editing.
	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+created.ID.String(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var fetched inspectionhttp.Response
	decodeJSON(t, resp, &fetched)
	if fetched.Type != "QUEEN" {
		t.Fatalf("get: type = %q, want %q", fetched.Type, "QUEEN")
	}

	// List for the hive
	resp = stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/inspections", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list pagination.Response[inspectionhttp.Response]
	decodeJSON(t, resp, &list)
	if len(list.Items) != 1 {
		t.Fatalf("list: got %d inspections, want 1", len(list.Items))
	}
	if list.Pagination.Total != 1 || list.Pagination.Page != 1 || list.Pagination.Limit != pagination.DefaultLimit {
		t.Fatalf("list: pagination = %+v, want total=1 page=1 limit=%d", list.Pagination, pagination.DefaultLimit)
	}

	// Update - changes the inspection type from "QUEEN" to "BROOD".
	resp = stack.request(t, http.MethodPut, "/api/v1/inspections/"+created.ID.String(), token, map[string]string{
		"inspected_at": "2026-03-16T09:00:00Z",
		"notes":        "re-inspected: all good",
		"type":         "BROOD",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated inspectionhttp.Response
	decodeJSON(t, resp, &updated)
	if updated.Notes != "re-inspected: all good" {
		t.Fatalf("update: notes = %q, want %q", updated.Notes, "re-inspected: all good")
	}
	if updated.Type != "BROOD" || updated.TypeLabel != "Brood" {
		t.Fatalf("update: type = %q, type_label = %q, want %q, %q", updated.Type, updated.TypeLabel, "BROOD", "Brood")
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
		"type":         "ROUTINE",
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
		"type":         "ROUTINE",
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
		{http.MethodPut, map[string]string{"inspected_at": testInspectedAt, "notes": "hijacked", "type": "ROUTINE"}},
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
	var list pagination.Response[inspectionhttp.Response]
	decodeJSON(t, resp, &list)
	if len(list.Items) != 0 {
		t.Fatalf("other user's list = %v, want empty", list.Items)
	}
	if list.Pagination.Total != 0 {
		t.Fatalf("other user's list total = %d, want 0", list.Pagination.Total)
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
		"type":         "ROUTINE",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with empty notes: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      "not-a-uuid",
		"inspected_at": testInspectedAt,
		"notes":        "ok",
		"type":         "ROUTINE",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with malformed hive_id: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      uuid.New().String(),
		"inspected_at": "not-a-date",
		"notes":        "ok",
		"type":         "ROUTINE",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with malformed inspected_at: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id":      uuid.New().String(),
		"inspected_at": testInspectedAt,
		"notes":        "ok",
		"type":         "swarm",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with invalid type: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
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

func TestInspectionFlow_ListPagination(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	for i := 0; i < 3; i++ {
		resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
			"hive_id":      hiveID.String(),
			"inspected_at": testInspectedAt,
			"notes":        "n/a",
			"type":         "ROUTINE",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %d: status = %d, want %d", i, resp.StatusCode, http.StatusCreated)
		}
	}

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/inspections?page=1&limit=2", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var page pagination.Response[inspectionhttp.Response]
	decodeJSON(t, resp, &page)
	if len(page.Items) != 2 {
		t.Fatalf("list page 1: got %d items, want 2", len(page.Items))
	}
	if page.Pagination.Total != 3 || page.Pagination.TotalPages != 2 || !page.Pagination.HasNext || page.Pagination.HasPrevious {
		t.Fatalf("list page 1: pagination = %+v, want total=3 total_pages=2 has_next=true has_previous=false", page.Pagination)
	}
}

// TestInspectionFlow_DeleteByHive is the end-to-end proof of the cascade
// primitive hive-service calls when it deletes a hive: every inspection
// under that hive is hard-deleted, while inspections under other hives
// (even the same user's) survive.
func TestInspectionFlow_DeleteByHive(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveA)

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id": hiveA.String(), "inspected_at": testInspectedAt, "notes": "in hiveA", "type": "ROUTINE",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create in hiveA: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var inHiveA inspectionhttp.Response
	decodeJSON(t, resp, &inHiveA)

	// Second token so the fake hive-service will authorize creating under
	// hiveB too - both inspections still belong to the same userID.
	stack.hive.allow(token, hiveB)
	resp = stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]string{
		"hive_id": hiveB.String(), "inspected_at": testInspectedAt, "notes": "in hiveB", "type": "ROUTINE",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create in hiveB: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var inHiveB inspectionhttp.Response
	decodeJSON(t, resp, &inHiveB)

	resp = stack.request(t, http.MethodDelete, "/api/v1/hives/"+hiveA.String()+"/inspections", token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteByHive: status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+inHiveA.ID.String(), token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get hiveA inspection after DeleteByHive: status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+inHiveB.ID.String(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get hiveB inspection after DeleteByHive on hiveA: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Calling it again for the same (now-empty) hive is a no-op, not an
	// error.
	resp = stack.request(t, http.MethodDelete, "/api/v1/hives/"+hiveA.String()+"/inspections", token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteByHive again on empty hive: status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestInspectionFlow_ListInvalidPageAndLimit(t *testing.T) {
	stack := newTestStack(t)
	token := stack.tokenFor(t, uuid.New())
	hiveID := uuid.New()

	cases := []string{
		"/api/v1/hives/" + hiveID.String() + "/inspections?page=0",
		"/api/v1/hives/" + hiveID.String() + "/inspections?page=-1",
		"/api/v1/hives/" + hiveID.String() + "/inspections?page=abc",
		"/api/v1/hives/" + hiveID.String() + "/inspections?limit=0",
		"/api/v1/hives/" + hiveID.String() + "/inspections?limit=101",
		"/api/v1/hives/" + hiveID.String() + "/inspections?limit=abc",
	}
	for _, path := range cases {
		resp := stack.request(t, http.MethodGet, path, token, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// TestInspectionFlow_PhotosAttachOnCreateAndDetachOnUpdate is the
// end-to-end proof of photo support: a caller can attach photos when
// creating an inspection, see them come back on Get, and later remove
// them via Update - exercised against a real cross-service HTTP call to
// (a fake) media-service, mirroring hive-service's own image flow.
func TestInspectionFlow_PhotosAttachOnCreateAndDetachOnUpdate(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	photo1 := uuid.New()
	photo2 := uuid.New()
	stack.media.own(photo1, photo2)

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]any{
		"hive_id":      hiveID.String(),
		"inspected_at": testInspectedAt,
		"notes":        "queen seen, brood pattern good",
		"type":         "QUEEN",
		"images":       []string{photo1.String(), photo2.String()},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create with photos: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created inspectionhttp.Response
	decodeJSON(t, resp, &created)
	if len(created.Images) != 2 {
		t.Fatalf("create: images = %v, want 2 entries", created.Images)
	}
	if !stack.media.calledWithQueryValue("ids", photo1.String()) {
		t.Error("create did not verify photo1's ownership against media-service")
	}

	// Get reflects the same attached photos.
	resp = stack.request(t, http.MethodGet, "/api/v1/inspections/"+created.ID.String(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var fetched inspectionhttp.Response
	decodeJSON(t, resp, &fetched)
	if len(fetched.Images) != 2 || fetched.Images[0].ImageURL == "" {
		t.Fatalf("get: images = %v, want 2 entries with non-empty image_url", fetched.Images)
	}

	// Update with an empty images array detaches every photo (references
	// only - it must not ask media-service to delete the underlying
	// files).
	resp = stack.request(t, http.MethodPut, "/api/v1/inspections/"+created.ID.String(), token, map[string]any{
		"inspected_at": testInspectedAt,
		"notes":        "re-inspected",
		"type":         "QUEEN",
		"images":       []string{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update clearing images: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated inspectionhttp.Response
	decodeJSON(t, resp, &updated)
	if len(updated.Images) != 0 {
		t.Fatalf("update: images = %v, want empty", updated.Images)
	}
	if stack.media.calledDeleteWithQueryValue("ids", photo1.String()) {
		t.Error("update clearing images must not delete the underlying media file")
	}
}

// TestInspectionFlow_PhotosRejectForeignMedia proves an inspection can't
// reference a media id that doesn't belong to the caller, verified
// against a real cross-service HTTP call.
func TestInspectionFlow_PhotosRejectForeignMedia(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)
	foreignPhoto := uuid.New() // deliberately never own()'d

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]any{
		"hive_id":      hiveID.String(),
		"inspected_at": testInspectedAt,
		"notes":        "snooping",
		"type":         "ROUTINE",
		"images":       []string{foreignPhoto.String()},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with foreign media: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	errBody, _ := body["error"].(map[string]any)
	fields, _ := errBody["fields"].(map[string]any)
	if fields["images"] != "image_not_found" {
		t.Fatalf("error fields = %v, want images: image_not_found", fields)
	}
}

// TestInspectionFlow_DeleteByHive_DeletesAttachedMedia proves the
// DeleteByHive cascade hard-deletes every media file referenced by the
// inspections it removes, not just the inspection rows themselves.
func TestInspectionFlow_DeleteByHive_DeletesAttachedMedia(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	photo := uuid.New()
	stack.media.own(photo)

	resp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]any{
		"hive_id":      hiveID.String(),
		"inspected_at": testInspectedAt,
		"notes":        "with a photo",
		"type":         "ROUTINE",
		"images":       []string{photo.String()},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	resp = stack.request(t, http.MethodDelete, "/api/v1/hives/"+hiveID.String()+"/inspections", token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteByHive: status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	if !stack.media.calledDeleteWithQueryValue("ids", photo.String()) {
		t.Error("DeleteByHive did not hard-delete the inspection's attached media")
	}
}
