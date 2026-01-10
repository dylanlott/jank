package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

//go:embed templates/*.html
var templatesFS embed.FS

var (
	db        *sql.DB
	templates *template.Template
	log       = logrus.New()
	auth      AuthConfig
)

func init() {
	// Set log format to JSON for production
	log.SetFormatter(&logrus.JSONFormatter{})
	// Set log level to info
	log.SetLevel(logrus.InfoLevel)
}

// ------------------- Data Models -------------------

// Board represents a message board.
type Board struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Threads     []*Thread `json:"threads,omitempty"`
}

// User represents a forum user.
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Created      time.Time `json:"created"`
}

// Thread represents a discussion thread on a board.
type Thread struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Author  string    `json:"author"`
	Posts   []*Post   `json:"posts,omitempty"`
	Created time.Time `json:"created"`
}

// Post represents an individual post in a thread.
type Post struct {
	ID      int       `json:"id"`
	Author  string    `json:"author"`
	Content string    `json:"content"`
	Created time.Time `json:"created"`
	Number  *big.Int  `json:"number"`
	Flair   string    `json:"flair"`
}

// ------------------- Template Data -------------------

// IndexViewData holds data for the index.html template.
type IndexViewData struct {
	AuthViewData
	Title       string
	Description string
	Boards      []*Board
}

// BoardViewData holds data for the board.html template.
type BoardViewData struct {
	AuthViewData
	Board *Board
}

// ThreadViewData holds data for the thread.html template.
type ThreadViewData struct {
	AuthViewData
	Thread  *Thread
	BoardID int
}

// NewThreadViewData holds data for the new_thread.html template.
type NewThreadViewData struct {
	AuthViewData
	BoardID int
}

// ProfileViewData holds data for the profile.html template.
type ProfileViewData struct {
	AuthViewData
	User    *User
	Threads []*ProfileThread
	Posts   []*ProfilePost
}

// PublicProfileViewData holds data for the public profile page.
type PublicProfileViewData struct {
	AuthViewData
	User    *User
	Threads []*ProfileThread
	Posts   []*ProfilePost
}

// UserLookupViewData holds data for the username lookup page.
type UserLookupViewData struct {
	AuthViewData
	Error string
}

// LoginViewData holds data for the login.html template.
type LoginViewData struct {
	AuthViewData
	Next  string
	Error string
}

// SignupViewData holds data for the signup.html template.
type SignupViewData struct {
	AuthViewData
	Next  string
	Error string
}

// ProfileThread is a lightweight thread view for profiles.
type ProfileThread struct {
	ID      int
	BoardID int
	Title   string
	Created time.Time
}

// ProfilePost is a lightweight post view for profiles.
type ProfilePost struct {
	ID          int
	ThreadID    int
	ThreadTitle string
	Content     string
	Created     time.Time
}

// AuthViewData holds shared auth template values.
type AuthViewData struct {
	IsAuthenticated bool
	Username        string
	CurrentPath     string
}

// AuthConfig holds credentials and signing secret for auth cookies.
type AuthConfig struct {
	Username  string
	Password  string
	Secret    []byte
	JWTSecret []byte
}

const authCookieName = "jank_auth"

// ------------------- main() & Initialization -------------------

func main() {
	var err error

	// 1. Open or create SQLite database
	db, err = sql.Open("sqlite3", "./sqlite.db")
	if err != nil {
		log.Fatalf("Failed to open SQLite DB: %v", err)
	}
	defer db.Close()

	// 2. Run migrations
	if err := migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// 3. Seed initial data (optional)
	if err := seedData(db); err != nil {
		log.Printf("Failed to seed data: %v", err)
	}

	// 4. Parse our embedded templates
	templates, err = template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// 5. Load auth config
	auth = loadAuthConfig()

	// 6. Ensure seed user exists
	if err := ensureSeedUser(db, auth.Username, auth.Password); err != nil {
		log.Fatalf("Failed to ensure seed user: %v", err)
	}

	// 7. Set up HTTP routes using gorilla/mux
	r := mux.NewRouter()

	// -- HTML pages --
	r.HandleFunc("/", serveIndex).Methods("GET")
	r.HandleFunc("/view/board/{boardID:[0-9]+}", serveBoardView).Methods("GET")
	r.HandleFunc("/view/board/newthread/{boardID:[0-9]+}", serveNewThread).Methods("GET", "POST")
	r.HandleFunc("/view/thread/{threadID:[0-9]+}", serveThreadView).Methods("GET")
	r.HandleFunc("/view/thread/{threadID:[0-9]+}/post", serveThreadView).Methods("POST")
	r.HandleFunc("/login", serveLogin).Methods("GET", "POST")
	r.HandleFunc("/signup", serveSignup).Methods("GET", "POST")
	r.HandleFunc("/logout", serveLogout).Methods("POST", "GET")
	r.HandleFunc("/profile", serveProfile).Methods("GET")
	r.HandleFunc("/user", serveUserLookup).Methods("GET", "POST")
	r.HandleFunc("/user/{username}", servePublicProfile).Methods("GET")
	r.HandleFunc("/auth/token", authTokenHandler).Methods("POST")
	r.HandleFunc("/auth/signup", authSignupHandler).Methods("POST")

	// -- REST API endpoints --
	r.HandleFunc("/boards", boardsHandler).Methods("GET", "POST")
	r.HandleFunc("/boards/{boardID:[0-9]+}", boardHandler).Methods("GET")
	r.HandleFunc("/threads/{boardID:[0-9]+}", threadsHandler).Methods("GET", "POST")
	r.HandleFunc("/posts/{boardID:[0-9]+}/{threadID:[0-9]+}", postsHandler).Methods("POST")
	r.HandleFunc("/delete/board/{boardID:[0-9]+}", deleteBoardHandler).Methods("DELETE")

	// 8. Start the server
	log.Info("Server listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ------------------- HTML Handlers -------------------

// serveIndex executes index.html, showing a list of boards with links.
func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Load all boards from DB
	boards, err := getAllBoards(db)
	if err != nil {
		log.Errorf("Failed to retrieve boards: %v", err)
		http.Error(w, "Failed to retrieve boards", http.StatusInternalServerError)
		return
	}

	// Prepare the template data
	authData := getAuthViewData(r)
	data := IndexViewData{
		AuthViewData: authData,
		Title:        "Welcome to /jank/",
		Description:  "Select a board below to view its threads.",
		Boards:       boards,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveBoardView executes board.html for a specific board (by ID).
func serveBoardView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid board ID", http.StatusBadRequest)
		return
	}

	// Load board + threads
	board, err := getBoardByID(db, boardID, true)
	if err != nil {
		log.Errorf("Board not found: %v", err)
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}

	authData := getAuthViewData(r)
	data := BoardViewData{
		AuthViewData: authData,
		Board:        board,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "board.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveNewThread lets a user create a new thread for a specific board.
//
// GET => Show the form (new_thread.html)
// POST => Process form data & create the thread, then redirect to the board view
func serveNewThread(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid board ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Just serve the form
		authData := getAuthViewData(r)
		data := NewThreadViewData{
			AuthViewData: authData,
			BoardID:      boardID,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "new_thread.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case http.MethodPost:
		if !requireAuth(w, r) {
			return
		}
		username, _ := getAuthenticatedUsername(r)
		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			http.Error(w, "Thread title cannot be empty", http.StatusBadRequest)
			return
		}

		// Create the thread
		thread, err := createThread(db, boardID, title, username)
		if err != nil {
			log.Errorf("Failed to create thread: %v", err)
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			return
		}

		// Log the created thread for debugging
		log.Infof("Created thread: ID=%d, Title=%s, BoardID=%d", thread.ID, thread.Title, boardID)

		// Redirect back to the board view
		http.Redirect(w, r, fmt.Sprintf("/view/board/%d", boardID), http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// serveThreadView handles both displaying a thread and adding new posts.
//
// GET => Display thread.html with thread and posts
// POST => Add a new post to the thread and redirect back to thread view
func serveThreadView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadIDStr := vars["threadID"]
	threadID, err := strconv.Atoi(threadIDStr)
	if err != nil {
		http.Error(w, "Invalid thread ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Handle GET request to view the thread

		// Fetch thread with posts
		thread, boardID, err := getThreadByID(db, threadID)
		if err != nil {
			log.Errorf("Thread not found: %v", err)
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}

		authData := getAuthViewData(r)
		data := ThreadViewData{
			AuthViewData: authData,
			Thread:       thread,
			BoardID:      boardID,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "thread.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	} else if r.Method == http.MethodPost {
		if !requireAuth(w, r) {
			return
		}
		username, _ := getAuthenticatedUsername(r)
		// Handle POST request to add a new post to the thread

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			http.Error(w, "Post content cannot be empty", http.StatusBadRequest)
			return
		}

		author := username
		// Add the post to the thread
		post, err := createPost(db, threadID, author, content)
		if err != nil {
			log.Errorf("Failed to create post: %v", err)
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			return
		}

		// Log the created post for debugging
		log.Infof("Created post: ID=%d, Author=%s, ThreadID=%d", post.ID, post.Author, threadID)

		// Redirect back to the thread view
		http.Redirect(w, r, fmt.Sprintf("/view/thread/%d", threadID), http.StatusSeeOther)

	} else {
		http.NotFound(w, r)
	}
}

// ------------------- REST Handlers (JSON) -------------------

// boardsHandler handles creation/listing of boards (REST API).
func boardsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		boards, err := getAllBoards(db)
		if err != nil {
			log.Errorf("Failed to retrieve boards: %v", err)
			http.Error(w, "Failed to retrieve boards", http.StatusInternalServerError)
			return
		}
		respondJSON(w, boards)

	case http.MethodPost:
		if !requireAPIAuth(w, r) {
			return
		}
		var board Board
		if err := json.NewDecoder(r.Body).Decode(&board); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		insertedBoard, err := createBoard(db, board.Name, board.Description)
		if err != nil {
			log.Errorf("Failed to create board: %v", err)
			http.Error(w, "Failed to create board", http.StatusInternalServerError)
			return
		}
		respondJSON(w, insertedBoard)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// boardHandler fetches a specific board (with threads + posts) in JSON form.
func boardHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		board, err := getBoardByID(db, boardID, true)
		if err != nil {
			log.Errorf("Board not found: %v", err)
			http.Error(w, "Board not found", http.StatusNotFound)
			return
		}
		respondJSON(w, board)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// threadsHandler lists or creates threads under a board (REST API).
func threadsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}
	log.Printf("handling threads for board %d", boardID)

	switch r.Method {
	case http.MethodGet:
		threads, err := getThreadsByBoardID(db, boardID, false)
		if err != nil {
			log.Errorf("Failed to retrieve threads: %v", err)
			http.Error(w, "Failed to retrieve threads", http.StatusInternalServerError)
			return
		}
		respondJSON(w, threads)

	case http.MethodPost:
		if !requireAPIAuth(w, r) {
			return
		}
		username, _ := getBearerUsername(r)
		var thread Thread
		if err := json.NewDecoder(r.Body).Decode(&thread); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("created thread %+v", &thread)

		insertedThread, err := createThread(db, boardID, thread.Title, username)
		if err != nil {
			log.Errorf("Failed to create thread: %v", err)
			http.Error(w, "Failed to create thread", http.StatusInternalServerError)
			return
		}
		respondJSON(w, insertedThread)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// postsHandler creates new posts in a given thread (REST API).
func postsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	threadIDStr := vars["threadID"]
	_, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}
	threadID, err := strconv.Atoi(threadIDStr)
	if err != nil {
		http.Error(w, "Invalid Thread ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if !requireAPIAuth(w, r) {
			return
		}
		username, _ := getBearerUsername(r)
		var post Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		post.Author = username
		insertedPost, err := createPost(db, threadID, post.Author, post.Content)
		if err != nil {
			log.Errorf("Failed to create post: %v", err)
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			return
		}
		respondJSON(w, insertedPost)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// deleteBoardHandler deletes a specific board by ID (REST API).
func deleteBoardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAPIAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}

	err = deleteBoardByID(db, boardID)
	if err != nil {
		log.Errorf("Failed to delete board: %v", err)
		http.Error(w, "Failed to delete board", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ------------------- Database & Utility -------------------

// migrate creates the necessary tables if they don't exist.
func migrate(db *sql.DB) error {
	boardsStmt := `
	CREATE TABLE IF NOT EXISTS boards (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT
	);`
	usersStmt := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created DATETIME NOT NULL
	);`
	threadsStmt := `
	CREATE TABLE IF NOT EXISTS threads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		board_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		author TEXT,
		created DATETIME NOT NULL,
		FOREIGN KEY (board_id) REFERENCES boards(id)
	);`
	postsStmt := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		thread_id INTEGER NOT NULL,
		author TEXT,
		content TEXT NOT NULL,
		created DATETIME NOT NULL,
		number TEXT,
		flair TEXT,
		FOREIGN KEY (thread_id) REFERENCES threads(id)
	);`

	if _, err := db.Exec(boardsStmt); err != nil {
		return err
	}
	if _, err := db.Exec(usersStmt); err != nil {
		return err
	}
	if _, err := db.Exec(threadsStmt); err != nil {
		return err
	}
	if err := ensureThreadsAuthorColumn(db); err != nil {
		return err
	}
	if _, err := db.Exec(postsStmt); err != nil {
		return err
	}
	return nil
}

// seedData inserts a default board if none exist.
func seedData(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM boards").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err := db.Exec(`INSERT INTO boards (name, description) VALUES (?, ?)`, "/test/", "A test board.")
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureThreadsAuthorColumn(db *sql.DB) error {
	_, err := db.Exec(`ALTER TABLE threads ADD COLUMN author TEXT`)
	if err == nil {
		_, _ = db.Exec(`UPDATE threads SET author = '' WHERE author IS NULL`)
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "duplicate column") || strings.Contains(lower, "already exists") {
		_, _ = db.Exec(`UPDATE threads SET author = '' WHERE author IS NULL`)
		return nil
	}
	return err
}

// ensureSeedUser creates a default user when none exists for the configured username.
func ensureSeedUser(db *sql.DB, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	if userExists(db, username) {
		return nil
	}
	_, err := createUser(db, username, password)
	return err
}

// createBoard inserts a new board into the database.
func createBoard(db *sql.DB, name, description string) (*Board, error) {
	result, err := db.Exec(`INSERT INTO boards (name, description) VALUES (?, ?)`, name, description)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Board{
		ID:          int(id),
		Name:        name,
		Description: description,
		Threads:     []*Thread{},
	}, nil
}

// getAllBoards retrieves all boards from the database.
func getAllBoards(db *sql.DB) ([]*Board, error) {
	rows, err := db.Query(`SELECT id, name, description FROM boards`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []*Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.Description); err != nil {
			return nil, err
		}
		boards = append(boards, &b)
	}
	return boards, nil
}

// getBoardByID retrieves a specific board by ID, optionally loading its threads.
func getBoardByID(db *sql.DB, boardID int, loadThreads bool) (*Board, error) {
	var b Board
	err := db.QueryRow(`SELECT id, name, description FROM boards WHERE id = ?`, boardID).
		Scan(&b.ID, &b.Name, &b.Description)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("board not found")
	} else if err != nil {
		return nil, err
	}

	if loadThreads {
		threads, err := getThreadsByBoardID(db, boardID, true)
		if err != nil {
			return nil, err
		}
		b.Threads = threads
	}
	return &b, nil
}

func userExists(db *sql.DB, username string) bool {
	var id int
	err := db.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err == sql.ErrNoRows {
		return false
	}
	return err == nil
}

func createUser(db *sql.DB, username, password string) (*User, error) {
	if userExists(db, username) {
		return nil, fmt.Errorf("username already exists")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := db.Exec(`INSERT INTO users (username, password_hash, created) VALUES (?, ?, ?)`, username, passwordHash, now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &User{
		ID:           int(id),
		Username:     username,
		PasswordHash: passwordHash,
		Created:      now,
	}, nil
}

func getUserPasswordHash(db *sql.DB, username string) (string, error) {
	var passwordHash string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, username).Scan(&passwordHash)
	if err != nil {
		return "", err
	}
	return passwordHash, nil
}

func getUserByUsername(db *sql.DB, username string) (*User, error) {
	var user User
	err := db.QueryRow(`SELECT id, username, password_hash, created FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Created)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func getThreadsByAuthor(db *sql.DB, username string) ([]*ProfileThread, error) {
	rows, err := db.Query(`
		SELECT t.id, t.board_id, t.title, t.created
		FROM threads t
		LEFT JOIN (
			SELECT thread_id, MIN(created) AS first_created
			FROM posts
			GROUP BY thread_id
		) fp ON fp.thread_id = t.id
		LEFT JOIN posts fp_post
			ON fp_post.thread_id = t.id AND fp_post.created = fp.first_created
		WHERE t.author = ?
			OR ((t.author IS NULL OR t.author = '') AND fp_post.author = ?)
		ORDER BY t.created DESC`, username, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*ProfileThread
	for rows.Next() {
		var t ProfileThread
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Created); err != nil {
			return nil, err
		}
		threads = append(threads, &t)
	}
	return threads, nil
}

func getPostsByAuthor(db *sql.DB, username string) ([]*ProfilePost, error) {
	rows, err := db.Query(`
		SELECT posts.id, posts.thread_id, threads.title, posts.content, posts.created
		FROM posts
		JOIN threads ON posts.thread_id = threads.id
		WHERE posts.author = ?
		ORDER BY posts.created DESC`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*ProfilePost
	for rows.Next() {
		var p ProfilePost
		if err := rows.Scan(&p.ID, &p.ThreadID, &p.ThreadTitle, &p.Content, &p.Created); err != nil {
			return nil, err
		}
		posts = append(posts, &p)
	}
	return posts, nil
}

func authenticateUser(db *sql.DB, username, password string) bool {
	if username == "" || password == "" {
		return false
	}
	passwordHash, err := getUserPasswordHash(db, username)
	if err != nil {
		return false
	}
	return verifyPassword(password, passwordHash)
}

// createThread inserts a new thread into the database.
func createThread(db *sql.DB, boardID int, title, author string) (*Thread, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO threads (board_id, title, author, created) 
		VALUES (?, ?, ?, ?)`,
		boardID, title, author, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Thread{
		ID:      int(id),
		Title:   title,
		Author:  author,
		Posts:   []*Post{},
		Created: now,
	}, nil
}

// getThreadsByBoardID retrieves all threads for a specific board, optionally loading their posts.
func getThreadsByBoardID(db *sql.DB, boardID int, loadPosts bool) ([]*Thread, error) {
	rows, err := db.Query(`
		SELECT id, title, author, created
		FROM threads
		WHERE board_id = ?
		ORDER BY created DESC`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*Thread
	for rows.Next() {
		var t Thread
		var author sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &author, &t.Created); err != nil {
			return nil, err
		}
		t.Author = author.String

		if loadPosts {
			posts, err := getPostsByThreadID(db, t.ID)
			if err != nil {
				return nil, err
			}
			t.Posts = posts
		}
		threads = append(threads, &t)
	}
	return threads, nil
}

// getThreadByID retrieves a specific thread by ID, along with its posts and board ID.
func getThreadByID(db *sql.DB, threadID int) (*Thread, int, error) {
	var t Thread
	var boardID int
	var author sql.NullString
	err := db.QueryRow(`SELECT id, board_id, title, author, created FROM threads WHERE id = ?`, threadID).
		Scan(&t.ID, &boardID, &t.Title, &author, &t.Created)
	if err == sql.ErrNoRows {
		return nil, 0, fmt.Errorf("thread not found")
	} else if err != nil {
		return nil, 0, err
	}
	t.Author = author.String

	// Fetch posts
	posts, err := getPostsByThreadID(db, threadID)
	if err != nil {
		return nil, 0, err
	}
	t.Posts = posts

	return &t, boardID, nil
}

// createPost inserts a new post into the database.
func createPost(db *sql.DB, threadID int, author, content string) (*Post, error) {
	now := time.Now()
	number, flair := generateUniqueNumberAndFlair()
	result, err := db.Exec(`
		INSERT INTO posts (thread_id, author, content, created, number, flair) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		threadID, author, content, now, number.String(), flair)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Post{
		ID:      int(id),
		Author:  author,
		Content: content,
		Created: now,
		Number:  number,
		Flair:   flair,
	}, nil
}

// generateUniqueNumberAndFlair generates a unique random large number and assigns a flair based on the number of preceding zeroes.
func generateUniqueNumberAndFlair() (*big.Int, string) {
	number, _ := rand.Int(rand.Reader, big.NewInt(1e10))
	numberStr := fmt.Sprintf("%d", number)

	// Count the number of preceding zeroes
	zeroCount := 0
	for _, char := range numberStr {
		if char == '0' {
			zeroCount++
		} else {
			break
		}
	}

	var flair string
	switch zeroCount {
	case 1:
		flair = "uno"
	case 2:
		flair = "dubs"
	case 3:
		flair = "trips"
	case 4:
		flair = "quads"
	case 5:
		flair = "pents"
	default:
		flair = "default"
	}

	return number, flair
}

// getPostsByThreadID retrieves all posts for a specific thread.
func getPostsByThreadID(db *sql.DB, threadID int) ([]*Post, error) {
	rows, err := db.Query(`
		SELECT id, author, content, created, number, flair
		FROM posts
		WHERE thread_id = ?
		ORDER BY created ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var p Post
		var numberStr string
		if err := rows.Scan(&p.ID, &p.Author, &p.Content, &p.Created, &numberStr, &p.Flair); err != nil {
			return nil, err
		}
		p.Number = new(big.Int)
		p.Number.SetString(numberStr, 10)
		posts = append(posts, &p)
	}
	return posts, nil
}

// deleteBoardByID deletes a board and its associated threads and posts from the database.
func deleteBoardByID(db *sql.DB, boardID int) error {
	_, err := db.Exec(`DELETE FROM boards WHERE id = ?`, boardID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM threads WHERE board_id = ?`, boardID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM posts WHERE thread_id IN (SELECT id FROM threads WHERE board_id = ?)`, boardID)
	return err
}

// respondJSON sends JSON responses (for our REST endpoints).
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return base64.RawURLEncoding.EncodeToString(salt) + ":" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func verifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	hash, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return hmac.Equal(hash, sum[:])
}

// ------------------- Auth Helpers -------------------

func loadAuthConfig() AuthConfig {
	username := strings.TrimSpace(os.Getenv("JANK_FORUM_USER"))
	password := strings.TrimSpace(os.Getenv("JANK_FORUM_PASS"))
	secret := strings.TrimSpace(os.Getenv("JANK_FORUM_SECRET"))
	jwtSecret := strings.TrimSpace(os.Getenv("JANK_JWT_SECRET"))

	if username == "" {
		username = "admin"
		log.Warn("JANK_FORUM_USER not set; defaulting to 'admin'")
	}
	if password == "" {
		password = "admin"
		log.Warn("JANK_FORUM_PASS not set; defaulting to 'admin'")
	}
	if secret == "" {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			log.Fatalf("Failed to generate auth secret: %v", err)
		}
		log.Warn("JANK_FORUM_SECRET not set; using a random secret for this process")
		config := AuthConfig{
			Username: username,
			Password: password,
			Secret:   secretBytes,
		}
		if jwtSecret == "" {
			jwtBytes := make([]byte, 32)
			if _, err := rand.Read(jwtBytes); err != nil {
				log.Fatalf("Failed to generate JWT secret: %v", err)
			}
			log.Warn("JANK_JWT_SECRET not set; using a random JWT secret for this process")
			config.JWTSecret = jwtBytes
		} else {
			config.JWTSecret = []byte(jwtSecret)
		}
		return config
	}

	if jwtSecret == "" {
		log.Warn("JANK_JWT_SECRET not set; defaulting to JANK_FORUM_SECRET")
		jwtSecret = secret
	}

	return AuthConfig{
		Username:  username,
		Password:  password,
		Secret:    []byte(secret),
		JWTSecret: []byte(jwtSecret),
	}
}

func getAuthViewData(r *http.Request) AuthViewData {
	username, ok := getAuthenticatedUsername(r)
	return AuthViewData{
		IsAuthenticated: ok,
		Username:        username,
		CurrentPath:     r.URL.RequestURI(),
	}
}

func getAuthenticatedUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return "", false
	}

	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		return "", false
	}

	username := parts[0]
	signature := parts[1]
	if username == "" || signature == "" {
		return "", false
	}

	expected := signAuthCookie(username)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", false
	}

	if !userExists(db, username) {
		return "", false
	}

	return username, true
}

func signAuthCookie(username string) string {
	mac := hmac.New(sha256.New, auth.Secret)
	_, _ = mac.Write([]byte(username))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, username string) {
	value := fmt.Sprintf("%s|%s", username, signAuthCookie(username))
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   60 * 60 * 24 * 7,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := getAuthenticatedUsername(r); ok {
		return true
	}

	next := r.URL.RequestURI()
	http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
	return false
}

func requireAPIAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := getBearerUsername(r); ok {
		return true
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func getBearerUsername(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return verifyJWT(parts[1])
}

func issueJWT(username string, ttl time.Duration) (string, time.Time, error) {
	if username == "" {
		return "", time.Time{}, fmt.Errorf("missing username")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	exp := time.Now().Add(ttl).Unix()
	payloadBytes, err := json.Marshal(map[string]interface{}{
		"sub": username,
		"exp": exp,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload

	mac := hmac.New(sha256.New, auth.JWTSecret)
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := unsigned + "." + signature
	return token, time.Unix(exp, 0), nil
}

func verifyJWT(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	unsigned := parts[0] + "." + parts[1]

	mac := hmac.New(sha256.New, auth.JWTSecret)
	_, _ = mac.Write([]byte(unsigned))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return "", false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	var payload struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", false
	}
	if payload.Sub == "" {
		return "", false
	}
	if time.Now().Unix() > payload.Exp {
		return "", false
	}
	if !userExists(db, payload.Sub) {
		return "", false
	}
	return payload.Sub, true
}

// ------------------- Auth Handlers -------------------

func serveLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		authData := getAuthViewData(r)
		next := sanitizeNext(r.URL.Query().Get("next"))
		data := LoginViewData{
			AuthViewData: authData,
			Next:         next,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "login.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		next := sanitizeNext(r.FormValue("next"))

		if authenticateUser(db, username, password) {
			setAuthCookie(w, r, username)
			if next == "" {
				next = "/"
			}
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}

		authData := getAuthViewData(r)
		data := LoginViewData{
			AuthViewData: authData,
			Next:         next,
			Error:        "Invalid username or password.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "login.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func serveLogout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	next := sanitizeNext(r.URL.Query().Get("next"))
	if next == "" {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func serveProfile(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	username, _ := getAuthenticatedUsername(r)
	user, err := getUserByUsername(db, username)
	if err != nil {
		http.Error(w, "Failed to load profile", http.StatusInternalServerError)
		return
	}
	threads, err := getThreadsByAuthor(db, username)
	if err != nil {
		http.Error(w, "Failed to load threads", http.StatusInternalServerError)
		return
	}
	posts, err := getPostsByAuthor(db, username)
	if err != nil {
		http.Error(w, "Failed to load posts", http.StatusInternalServerError)
		return
	}

	authData := getAuthViewData(r)
	data := ProfileViewData{
		AuthViewData: authData,
		User:         user,
		Threads:      threads,
		Posts:        posts,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "profile.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveUserLookup(w http.ResponseWriter, r *http.Request) {
	authData := getAuthViewData(r)
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			data := UserLookupViewData{
				AuthViewData: authData,
				Error:        "Please enter a username.",
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := templates.ExecuteTemplate(w, "user_lookup.html", data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.Redirect(w, r, "/user/"+url.PathEscape(username), http.StatusSeeOther)
		return
	}

	data := UserLookupViewData{AuthViewData: authData}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "user_lookup.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func servePublicProfile(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["username"]
	if strings.TrimSpace(username) == "" {
		http.NotFound(w, r)
		return
	}
	user, err := getUserByUsername(db, username)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	threads, err := getThreadsByAuthor(db, username)
	if err != nil {
		http.Error(w, "Failed to load threads", http.StatusInternalServerError)
		return
	}
	posts, err := getPostsByAuthor(db, username)
	if err != nil {
		http.Error(w, "Failed to load posts", http.StatusInternalServerError)
		return
	}

	authData := getAuthViewData(r)
	data := PublicProfileViewData{
		AuthViewData: authData,
		User:         user,
		Threads:      threads,
		Posts:        posts,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "public_profile.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func authTokenHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !authenticateUser(db, credentials.Username, credentials.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, expiresAt, err := issueJWT(credentials.Username, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func serveSignup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		authData := getAuthViewData(r)
		next := sanitizeNext(r.URL.Query().Get("next"))
		data := SignupViewData{
			AuthViewData: authData,
			Next:         next,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "signup.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		next := sanitizeNext(r.FormValue("next"))

		if username == "" || password == "" {
			renderSignupError(w, r, next, "Username and password are required.")
			return
		}
		if _, err := createUser(db, username, password); err != nil {
			log.Errorf("Failed to create user: %v", err)
			renderSignupError(w, r, next, signupErrorMessage(err))
			return
		}

		setAuthCookie(w, r, username)
		if next == "" {
			next = "/"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func renderSignupError(w http.ResponseWriter, r *http.Request, next, message string) {
	authData := getAuthViewData(r)
	data := SignupViewData{
		AuthViewData: authData,
		Next:         next,
		Error:        message,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "signup.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func signupErrorMessage(err error) string {
	if err == nil {
		return "Failed to create account."
	}
	if strings.Contains(strings.ToLower(err.Error()), "exists") {
		return "That username is already taken."
	}
	return "Failed to create account."
}

func authSignupHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}
	if _, err := createUser(db, credentials.Username, credentials.Password); err != nil {
		log.Errorf("Failed to create user: %v", err)
		http.Error(w, signupErrorMessage(err), http.StatusBadRequest)
		return
	}
	token, expiresAt, err := issueJWT(credentials.Username, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func sanitizeNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return ""
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return ""
	}
	return next
}
