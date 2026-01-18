package app

import (
	"math/big"
	"time"
)

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
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	Posts      []*Post   `json:"posts,omitempty"`
	Created    time.Time `json:"created"`
	Tags       []string  `json:"tags,omitempty"`
	ReplyCount int       `json:"-"`
	LastBump   time.Time `json:"-"`
	CardTags   []string  `json:"-"`
	Excerpt    string    `json:"excerpt,omitempty"`
}

// ThreadSearchResult represents a thread search hit with board context.
type ThreadSearchResult struct {
	ID        int
	BoardID   int
	BoardName string
	Title     string
	Author    string
	Created   time.Time
}

// CardTree represents a scoped tree of cards with annotations.
type CardTree struct {
	ID          int             `json:"id"`
	ScopeType   string          `json:"scope_type"`
	ScopeID     int             `json:"scope_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	IsPrimary   bool            `json:"is_primary"`
	Nodes       []*CardTreeNode `json:"nodes,omitempty"`
}

// CardTreeNode represents a card in a tree with optional annotations.
type CardTreeNode struct {
	ID          int                   `json:"id"`
	TreeID      int                   `json:"tree_id"`
	ParentID    *int                  `json:"parent_id,omitempty"`
	CardName    string                `json:"card_name"`
	Position    int                   `json:"position"`
	CreatedBy   string                `json:"created_by"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Depth       int                   `json:"depth,omitempty"`
	Indent      int                   `json:"indent,omitempty"`
	Annotations []*CardTreeAnnotation `json:"annotations,omitempty"`
}

// CardTreeAnnotation represents a note attached to a tree node.
type CardTreeAnnotation struct {
	ID           int       `json:"id"`
	NodeID       int       `json:"node_id"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	Label        string    `json:"label"`
	Tags         string    `json:"tags"`
	SourcePostID *int      `json:"source_post_id,omitempty"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// Post represents an individual post in a thread.
type Post struct {
	ID            int         `json:"id"`
	Author        string      `json:"author"`
	Content       string      `json:"content"`
	Created       time.Time   `json:"created"`
	Number        *big.Int    `json:"number"`
	Flair         string      `json:"flair"`
	Trees         []*CardTree `json:"trees,omitempty"`
	IsDeleted     bool        `json:"-"`
	DeletedAt     *time.Time  `json:"-"`
	DeletedBy     string      `json:"-"`
	DeletedReason string      `json:"-"`
}

// Klaxon represents a site-wide announcement banner.
type Klaxon struct {
	ID        int
	Tone      string
	Emoji     string
	Message   string
	UpdatedAt time.Time
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
	Thread                *Thread
	BoardID               int
	LastBump              time.Time
	BumpCooldownRemaining int
	NecroWarning          bool
	ReportCategories      []string
}

// NewThreadViewData holds data for the new_thread.html template.
type NewThreadViewData struct {
	AuthViewData
	BoardID int
}

// SearchViewData holds data for the search page.
type SearchViewData struct {
	AuthViewData
	Boards  []*Board
	Threads []*ThreadSearchResult
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

// ErrorViewData holds data for the error.html template.
type ErrorViewData struct {
	AuthViewData
	Title   string
	Message string
	BackURL string
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
	IsModerator     bool
	SearchQuery     string
	Klaxon          *Klaxon
}

// Report represents a moderation report.
type Report struct {
	ID             int        `json:"id"`
	PostID         int        `json:"post_id"`
	Category       string     `json:"category"`
	Reason         string     `json:"reason,omitempty"`
	ReportedBy     string     `json:"reported_by,omitempty"`
	Created        time.Time  `json:"created"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy     string     `json:"resolved_by,omitempty"`
	ResolutionNote string     `json:"resolution_note,omitempty"`
}

// ModReport is a report with joined post/thread context.
type ModReport struct {
	Report
	PostAuthor        string    `json:"post_author"`
	PostContent       string    `json:"post_content"`
	PostCreated       time.Time `json:"post_created"`
	PostDeleted       bool      `json:"post_deleted"`
	PostDeletedReason string    `json:"post_deleted_reason,omitempty"`
	ThreadID          int       `json:"thread_id"`
	ThreadTitle       string    `json:"thread_title"`
	BoardID           int       `json:"board_id"`
	BoardName         string    `json:"board_name"`
}

// ModReportsViewData holds data for the moderation queue page.
type ModReportsViewData struct {
	AuthViewData
	Reports []*ModReport
}

// KlaxonAdminViewData holds data for the klaxon admin page.
type KlaxonAdminViewData struct {
	AuthViewData
	Klaxon  *Klaxon
	Error   string
	Success string
}
