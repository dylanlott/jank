package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

//go:embed templates/*.html
var templatesFS embed.FS

var (
	db        *sql.DB
	templates *template.Template
	log       = logrus.New()
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

// Thread represents a discussion thread on a board.
type Thread struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Posts   []*Post   `json:"posts,omitempty"`
	Created time.Time `json:"created"`
}

// Post represents an individual post in a thread.
type Post struct {
	ID      int       `json:"id"`
	Author  string    `json:"author"`
	Content string    `json:"content"`
	Created time.Time `json:"created"`
}

// ------------------- Template Data -------------------

// IndexViewData holds data for the index.html template.
type IndexViewData struct {
	Title       string
	Description string
	Boards      []*Board
}

// BoardViewData holds data for the board.html template.
type BoardViewData struct {
	Board *Board
}

// ThreadViewData holds data for the thread.html template.
type ThreadViewData struct {
	Thread  *Thread
	BoardID int
}

// NewThreadViewData holds data for the new_thread.html template.
type NewThreadViewData struct {
	BoardID int
}

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

	// 5. Set up HTTP routes
	// -- HTML pages --
	http.HandleFunc("/", serveIndex)                          // Homepage
	http.HandleFunc("/view/board/", serveBoardView)           // Board detail page
	http.HandleFunc("/view/board/newthread/", serveNewThread) // New thread creation (GET/POST)
	http.HandleFunc("/view/thread/", serveThreadView)         // Individual thread view and post handling

	// -- REST API endpoints --
	http.HandleFunc("/boards", boardsHandler)
	http.HandleFunc("/boards/", boardHandler)
	http.HandleFunc("/threads/", threadsHandler)
	http.HandleFunc("/posts/", postsHandler)

	// 6. Start the server
	log.Info("Server listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
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
	data := IndexViewData{
		Title:       "Welcome to 4chan Clone",
		Description: "Select a board below to view its threads.",
		Boards:      boards,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveBoardView executes board.html for a specific board (by ID).
func serveBoardView(w http.ResponseWriter, r *http.Request) {
	// Example path: /view/board/1
	// We'll parse the board ID from the path.
	parts := strings.Split(r.URL.Path, "/")
	// parts = ["", "view", "board", "{boardID}"]
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	boardIDStr := parts[len(parts)-1]
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

	data := BoardViewData{Board: board}

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
	// Example path: /view/board/newthread/1
	// We'll parse the board ID from the path.
	parts := strings.Split(r.URL.Path, "/")
	// parts = ["", "view", "board", "newthread", "{boardID}"]
	if len(parts) < 5 {
		http.NotFound(w, r)
		return
	}
	boardIDStr := parts[len(parts)-1]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid board ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Just serve the form
		data := NewThreadViewData{BoardID: boardID}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "new_thread.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case http.MethodPost:
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
		thread, err := createThread(db, boardID, title)
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
	// Example paths:
	// GET: /view/thread/1
	// POST: /view/thread/1/post
	path := strings.TrimPrefix(r.URL.Path, "/view/thread/")
	parts := strings.Split(path, "/")

	if len(parts) == 1 && r.Method == http.MethodGet {
		// Handle GET request to view the thread
		threadIDStr := parts[0]
		threadID, err := strconv.Atoi(threadIDStr)
		if err != nil {
			http.Error(w, "Invalid thread ID", http.StatusBadRequest)
			return
		}

		// Fetch thread with posts
		thread, boardID, err := getThreadByID(db, threadID)
		if err != nil {
			log.Errorf("Thread not found: %v", err)
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}

		data := ThreadViewData{
			Thread:  thread,
			BoardID: boardID,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "thread.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	} else if len(parts) == 2 && parts[1] == "post" && r.Method == http.MethodPost {
		// Handle POST request to add a new post to the thread
		threadIDStr := parts[0]
		threadID, err := strconv.Atoi(threadIDStr)
		if err != nil {
			http.Error(w, "Invalid thread ID", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		author := strings.TrimSpace(r.FormValue("author"))
		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			http.Error(w, "Post content cannot be empty", http.StatusBadRequest)
			return
		}

		// Optionally, set a default author if none provided
		if author == "" {
			author = "Anonymous"
		}

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
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/boards/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	boardID, err := strconv.Atoi(parts[0])
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
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/threads/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	boardID, err := strconv.Atoi(parts[0])
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
		var thread Thread
		if err := json.NewDecoder(r.Body).Decode(&thread); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("created thread %+v", &thread)

		insertedThread, err := createThread(db, boardID, thread.Title)
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
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/posts/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid URL format. Must be /posts/{boardID}/{threadID}", http.StatusBadRequest)
		return
	}
	// boardID is parsed but not used; use '_' to ignore
	_, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}
	threadID, err := strconv.Atoi(parts[1])
	if err != nil {
		http.Error(w, "Invalid Thread ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var post Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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

// ------------------- Database & Utility -------------------

// migrate creates the necessary tables if they don't exist.
func migrate(db *sql.DB) error {
	boardsStmt := `
	CREATE TABLE IF NOT EXISTS boards (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT
	);`
	threadsStmt := `
	CREATE TABLE IF NOT EXISTS threads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		board_id INTEGER NOT NULL,
		title TEXT NOT NULL,
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
		FOREIGN KEY (thread_id) REFERENCES threads(id)
	);`

	if _, err := db.Exec(boardsStmt); err != nil {
		return err
	}
	if _, err := db.Exec(threadsStmt); err != nil {
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

// createThread inserts a new thread into the database.
func createThread(db *sql.DB, boardID int, title string) (*Thread, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO threads (board_id, title, created) 
		VALUES (?, ?, ?)`,
		boardID, title, now)
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
		Posts:   []*Post{},
		Created: now,
	}, nil
}

// getThreadsByBoardID retrieves all threads for a specific board, optionally loading their posts.
func getThreadsByBoardID(db *sql.DB, boardID int, loadPosts bool) ([]*Thread, error) {
	rows, err := db.Query(`
		SELECT id, title, created
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Created); err != nil {
			return nil, err
		}

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
	err := db.QueryRow(`SELECT id, board_id, title, created FROM threads WHERE id = ?`, threadID).
		Scan(&t.ID, &boardID, &t.Title, &t.Created)
	if err == sql.ErrNoRows {
		return nil, 0, fmt.Errorf("thread not found")
	} else if err != nil {
		return nil, 0, err
	}

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
	result, err := db.Exec(`
		INSERT INTO posts (thread_id, author, content, created) 
		VALUES (?, ?, ?, ?)`,
		threadID, author, content, now)
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
	}, nil
}

// getPostsByThreadID retrieves all posts for a specific thread.
func getPostsByThreadID(db *sql.DB, threadID int) ([]*Post, error) {
	rows, err := db.Query(`
		SELECT id, author, content, created
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
		if err := rows.Scan(&p.ID, &p.Author, &p.Content, &p.Created); err != nil {
			return nil, err
		}
		posts = append(posts, &p)
	}
	return posts, nil
}

// respondJSON sends JSON responses (for our REST endpoints).
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}
