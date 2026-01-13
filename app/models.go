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
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Author  string    `json:"author"`
	Posts   []*Post   `json:"posts,omitempty"`
	Created time.Time `json:"created"`
	ReplyCount int       `json:"-"`
	LastBump   time.Time `json:"-"`
	CardTags   []string  `json:"-"`
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
	ID      int       `json:"id"`
	Author  string    `json:"author"`
	Content string    `json:"content"`
	Created time.Time `json:"created"`
	Number  *big.Int  `json:"number"`
	Flair   string    `json:"flair"`
	Trees   []*CardTree `json:"trees,omitempty"`
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
}
