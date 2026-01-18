package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	testDB.SetMaxOpenConns(1)
	if _, err := testDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	dbDriver = "sqlite3"
	db = testDB
	auth = AuthConfig{
		Username:  "admin",
		Secret:    []byte("test-secret"),
		JWTSecret: []byte("test-jwt-secret"),
	}

	if err := migrate(testDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Cleanup(func() {
		_ = testDB.Close()
	})
	return testDB
}

func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	respondJSON(rec, payload)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"status": "ok"`)) {
		t.Fatalf("response body missing payload: %s", rec.Body.String())
	}
}

func TestAuthSignupHandler(t *testing.T) {
	setupTestDB(t)

	body := bytes.NewBufferString(`{"username":"alice","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/signup", body)
	rec := httptest.NewRecorder()

	authSignupHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !userExists(db, "alice") {
		t.Fatalf("expected user to be created")
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["token"] == "" {
		t.Fatalf("expected token in response")
	}
}

func TestAuthTokenHandler(t *testing.T) {
	setupTestDB(t)

	if _, err := createUser(db, "bob", "secret"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	body := bytes.NewBufferString(`{"username":"bob","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/token", body)
	rec := httptest.NewRecorder()

	authTokenHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["token"] == "" {
		t.Fatalf("expected token in response")
	}
}

func TestBoardsHandlerGet(t *testing.T) {
	setupTestDB(t)
	if err := seedData(db); err != nil {
		t.Fatalf("seed data: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/boards", nil)
	rec := httptest.NewRecorder()

	boardsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var boards []Board
	if err := json.NewDecoder(rec.Body).Decode(&boards); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(boards) == 0 {
		t.Fatalf("expected boards in response")
	}
}

func TestBoardsHandlerPostUnauthorized(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"name":"/go/","description":"golang"}`))
	rec := httptest.NewRecorder()

	boardsHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBoardsHandlerPostAuthorized(t *testing.T) {
	setupTestDB(t)

	if _, err := createUser(db, "carol", "secret"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := issueJWT("carol", time.Hour)
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"name":"/go/","description":"golang"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	boardsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var board Board
	if err := json.NewDecoder(rec.Body).Decode(&board); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if board.Name != "/go/" {
		t.Fatalf("expected board name /go/, got %q", board.Name)
	}
}

func TestTreeNodesHandlerMissingCardName(t *testing.T) {
	setupTestDB(t)

	if _, err := createUser(db, "dana", "secret"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := issueJWT("dana", time.Hour)
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/trees/1/nodes", bytes.NewBufferString(`{"position":1}`))
	req = mux.SetURLVars(req, map[string]string{"treeID": "1"})
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	treeNodesHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVerifyJWTExpired(t *testing.T) {
	setupTestDB(t)

	if _, err := createUser(db, "erin", "secret"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := issueJWT("erin", -1*time.Minute)
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}
	if _, ok := verifyJWT(token); ok {
		t.Fatalf("expected expired token to be rejected")
	}
}

func TestReportsAPIModerationFlow(t *testing.T) {
	setupTestDB(t)

	if _, err := createUser(db, "admin", "secret"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := createUser(db, "alice", "secret"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	board, err := createBoard(db, "/test/", "test board")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	thread, err := createThread(db, board.ID, "hello", "alice")
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	post, err := createPost(db, thread.ID, "alice", "nope")
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	userToken, _, err := issueJWT("alice", time.Hour)
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}

	reportBody := bytes.NewBufferString(`{"post_id":` + strconv.Itoa(post.ID) + `,"category":"spam","reason":"bad"}`)
	reportReq := httptest.NewRequest(http.MethodPost, "/reports", reportBody)
	reportReq.Header.Set("Authorization", "Bearer "+userToken)
	reportRec := httptest.NewRecorder()

	reportsHandler(reportRec, reportReq)

	if reportRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", reportRec.Code, reportRec.Body.String())
	}
	var reportResp Report
	if err := json.NewDecoder(reportRec.Body).Decode(&reportResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if reportResp.ID == 0 {
		t.Fatalf("expected report id")
	}

	modToken, _, err := issueJWT("admin", time.Hour)
	if err != nil {
		t.Fatalf("issue admin jwt: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/reports", nil)
	listReq.Header.Set("Authorization", "Bearer "+modToken)
	listRec := httptest.NewRecorder()

	reportsHandler(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var reports []ModReport
	if err := json.NewDecoder(listRec.Body).Decode(&reports); err != nil {
		t.Fatalf("decode reports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/reports/1/resolve", bytes.NewBufferString(`{"note":"handled"}`))
	resolveReq = mux.SetURLVars(resolveReq, map[string]string{"reportID": strconv.Itoa(reportResp.ID)})
	resolveReq.Header.Set("Authorization", "Bearer "+modToken)
	resolveRec := httptest.NewRecorder()

	reportResolveHandler(resolveRec, resolveReq)

	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resolveRec.Code, resolveRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/posts/1/delete", bytes.NewBufferString(`{"reason":"rules"}`))
	deleteReq = mux.SetURLVars(deleteReq, map[string]string{"postID": strconv.Itoa(post.ID)})
	deleteReq.Header.Set("Authorization", "Bearer "+modToken)
	deleteRec := httptest.NewRecorder()

	postDeleteHandler(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	posts, err := getPostsByThreadID(db, thread.ID)
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 1 || !posts[0].IsDeleted {
		t.Fatalf("expected post to be deleted")
	}
}
