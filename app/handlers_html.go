package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ------------------- HTML Handlers -------------------

var reportCategories = []string{
	"spam",
	"harassment",
	"illegal",
	"off-topic",
	"other",
}

// serveIndex executes index.html, showing a list of boards with links.
func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		renderErrorPage(w, r, http.StatusNotFound, "Not Found", "That page does not exist.", "/")
		return
	}

	boards, err := getAllBoards(db)
	if err != nil {
		log.Errorf("Failed to retrieve boards: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Boards Unavailable", "Failed to load boards. Please try again.", "/")
		return
	}

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

// serveSearch executes search.html with board and thread matches.
func serveSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	authData := getAuthViewData(r)
	authData.SearchQuery = query

	data := SearchViewData{
		AuthViewData: authData,
		Boards:       []*Board{},
		Threads:      []*ThreadSearchResult{},
	}

	if query != "" {
		boards, err := searchBoards(db, query, 20)
		if err != nil {
			log.Errorf("Failed to search boards: %v", err)
			renderErrorPage(w, r, http.StatusInternalServerError, "Search Unavailable", "Board search failed. Please try again.", "/")
			return
		}
		threads, err := searchThreads(db, query, 50)
		if err != nil {
			log.Errorf("Failed to search threads: %v", err)
			renderErrorPage(w, r, http.StatusInternalServerError, "Search Unavailable", "Thread search failed. Please try again.", "/")
			return
		}
		data.Boards = boards
		data.Threads = threads
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "search.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveBoardView executes board.html for a specific board (by ID).
func serveBoardView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Board", "That board ID is not valid.", "/")
		return
	}

	board, err := getBoardByID(db, boardID, true)
	if err != nil {
		log.Errorf("Board not found: %v", err)
		renderErrorPage(w, r, http.StatusNotFound, "Board Not Found", "We couldn't find that board.", "/")
		return
	}
	if board != nil {
		cardTagPattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
		for _, thread := range board.Threads {
			if thread == nil {
				continue
			}
			thread.ReplyCount = 0
			thread.LastBump = thread.Created
			thread.CardTags = nil

			if len(thread.Posts) == 0 {
				continue
			}

			if len(thread.Posts) > 1 {
				thread.ReplyCount = len(thread.Posts) - 1
			}
			thread.LastBump = thread.Posts[len(thread.Posts)-1].Created

			opContent := thread.Posts[0].Content
			matches := cardTagPattern.FindAllStringSubmatch(opContent, -1)
			if len(matches) == 0 {
				continue
			}
			seen := make(map[string]struct{})
			for _, match := range matches {
				tag := strings.TrimSpace(match[1])
				if tag == "" {
					continue
				}
				if _, ok := seen[tag]; ok {
					continue
				}
				seen[tag] = struct{}{}
				thread.CardTags = append(thread.CardTags, tag)
				if len(thread.CardTags) >= 4 {
					break
				}
			}
		}
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
func serveNewThread(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Board", "That board ID is not valid.", "/")
		return
	}

	switch r.Method {
	case http.MethodGet:
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
		if err := r.ParseForm(); err != nil {
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that form submission.", fmt.Sprintf("/view/board/%d", boardID))
			return
		}
		treePayload, err := parseCardTreePayload(r.FormValue("tree_payload"))
		if err != nil {
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Tree Data", "We couldn't read your card tree details.", fmt.Sprintf("/view/board/newthread/%d", boardID))
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			renderErrorPage(w, r, http.StatusBadRequest, "Missing Title", "Thread title cannot be empty.", fmt.Sprintf("/view/board/newthread/%d", boardID))
			return
		}
		tags, err := validateTags(parseTagsInput(r.FormValue("tags")))
		if err != nil {
			title := "Invalid Tags"
			message := "Tags must be short and limited in count."
			if errors.Is(err, errTagCount) {
				title = "Too Many Tags"
				message = fmt.Sprintf("Please keep tags to %d or fewer.", maxThreadTags)
			} else if errors.Is(err, errTagLength) {
				title = "Tag Too Long"
				message = fmt.Sprintf("Each tag must be %d characters or fewer.", maxTagLength)
			}
			renderErrorPage(w, r, http.StatusBadRequest, title, message, fmt.Sprintf("/view/board/newthread/%d", boardID))
			return
		}
		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			renderErrorPage(w, r, http.StatusBadRequest, "Missing Post", "Thread content cannot be empty.", fmt.Sprintf("/view/board/newthread/%d", boardID))
			return
		}

		thread, err := createThread(db, boardID, title, username, tags)
		if err != nil {
			log.Errorf("Failed to create thread: %v", err)
			renderErrorPage(w, r, http.StatusInternalServerError, "Create Thread Failed", "We couldn't create that thread. Please try again.", fmt.Sprintf("/view/board/%d", boardID))
			return
		}
		post, err := createPost(db, thread.ID, username, content)
		if err != nil {
			log.Errorf("Failed to create starter post: %v", err)
			renderErrorPage(w, r, http.StatusInternalServerError, "Post Failed", "We couldn't save your post. Please try again.", fmt.Sprintf("/view/board/%d", boardID))
			return
		}
		if err := applyCardTreePayload(db, "post", post.ID, username, treePayload); err != nil {
			log.Errorf("Failed to create card tree: %v", err)
			renderErrorPage(w, r, http.StatusBadRequest, "Tree Create Failed", "We couldn't save your card trees. Please review and try again.", fmt.Sprintf("/view/board/newthread/%d", boardID))
			return
		}

		log.Infof("Created thread: ID=%d, Title=%s, BoardID=%d", thread.ID, thread.Title, boardID)
		http.Redirect(w, r, fmt.Sprintf("/view/board/%d", boardID), http.StatusSeeOther)

	default:
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", fmt.Sprintf("/view/board/%d", boardID))
	}
}

// serveThreadView handles both displaying a thread and adding new posts.
func serveThreadView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadIDStr := vars["threadID"]
	threadID, err := strconv.Atoi(threadIDStr)
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Thread", "That thread ID is not valid.", "/")
		return
	}

	if r.Method == http.MethodGet {
		thread, boardID, err := getThreadByID(db, threadID)
		if err != nil {
			log.Errorf("Thread not found: %v", err)
			renderErrorPage(w, r, http.StatusNotFound, "Thread Not Found", "We couldn't find that thread.", "/")
			return
		}

		lastBump := thread.Created
		if len(thread.Posts) > 0 {
			lastBump = thread.Posts[len(thread.Posts)-1].Created
		}
		const bumpCooldown = 3 * time.Minute
		const necroThreshold = 30 * 24 * time.Hour
		sinceBump := time.Since(lastBump)
		bumpCooldownRemaining := 0
		if sinceBump >= 0 && sinceBump < bumpCooldown {
			bumpCooldownRemaining = int(bumpCooldown.Seconds() - sinceBump.Seconds())
		}
		necroWarning := sinceBump > necroThreshold
		authData := getAuthViewData(r)
		data := ThreadViewData{
			AuthViewData:          authData,
			Thread:                thread,
			BoardID:               boardID,
			LastBump:              lastBump,
			BumpCooldownRemaining: bumpCooldownRemaining,
			NecroWarning:          necroWarning,
			ReportCategories:      reportCategories,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, "thread.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		if !requireAuth(w, r) {
			return
		}
		username, _ := getAuthenticatedUsername(r)
		if err := r.ParseForm(); err != nil {
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that form submission.", fmt.Sprintf("/view/thread/%d", threadID))
			return
		}
		treePayload, err := parseCardTreePayload(r.FormValue("tree_payload"))
		if err != nil {
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Tree Data", "We couldn't read your card tree details.", fmt.Sprintf("/view/thread/%d", threadID))
			return
		}
		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			renderErrorPage(w, r, http.StatusBadRequest, "Missing Post", "Post content cannot be empty.", fmt.Sprintf("/view/thread/%d", threadID))
			return
		}

		author := username
		post, err := createPost(db, threadID, author, content)
		if err != nil {
			log.Errorf("Failed to create post: %v", err)
			renderErrorPage(w, r, http.StatusInternalServerError, "Post Failed", "We couldn't create that reply. Please try again.", fmt.Sprintf("/view/thread/%d", threadID))
			return
		}
		if err := applyCardTreePayload(db, "post", post.ID, username, treePayload); err != nil {
			log.Errorf("Failed to create card tree: %v", err)
			renderErrorPage(w, r, http.StatusBadRequest, "Tree Create Failed", "We couldn't save your card trees. Please review and try again.", fmt.Sprintf("/view/thread/%d", threadID))
			return
		}

		log.Infof("Created post: ID=%d, Author=%s, ThreadID=%d", post.ID, post.Author, threadID)
		http.Redirect(w, r, fmt.Sprintf("/view/thread/%d", threadID), http.StatusSeeOther)
		return
	}

	renderErrorPage(w, r, http.StatusNotFound, "Not Found", "That page does not exist.", "/")
}

func reportPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
		return
	}
	if !requireAuth(w, r) {
		return
	}
	vars := mux.Vars(r)
	postID, err := strconv.Atoi(vars["postID"])
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Post", "That post ID is not valid.", "/")
		return
	}
	if err := r.ParseForm(); err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that report.", "/")
		return
	}
	category := strings.TrimSpace(r.FormValue("category"))
	if !isValidReportCategory(category) {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Category", "Please pick a report category.", "/")
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	username, _ := getAuthenticatedUsername(r)
	if _, err := createReport(db, postID, category, reason, username); err != nil {
		log.Errorf("Failed to create report: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Report Failed", "We couldn't send that report.", "/")
		return
	}

	threadID, err := getPostThreadID(db, postID)
	if err != nil {
		renderErrorPage(w, r, http.StatusNotFound, "Post Not Found", "We couldn't find that post.", "/")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/view/thread/%d", threadID), http.StatusSeeOther)
}

func serveModReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
		return
	}
	if !requireModerator(w, r) {
		return
	}
	reports, err := getOpenReports(db)
	if err != nil {
		log.Errorf("Failed to load reports: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Queue Unavailable", "We couldn't load the report queue.", "/")
		return
	}

	authData := getAuthViewData(r)
	data := ModReportsViewData{
		AuthViewData: authData,
		Reports:      reports,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "mod_reports.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveKlaxonAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
		return
	}
	if !requireModerator(w, r) {
		return
	}

	var message string
	var success string

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that klaxon update.", "/mod/klaxon")
			return
		}
		if r.FormValue("clear") != "" {
			if err := saveKlaxon(db, "", "", "", time.Now()); err != nil {
				log.Errorf("Failed to clear klaxon: %v", err)
				message = "Failed to clear the klaxon."
			} else {
				success = "Klaxon cleared."
			}
		} else {
			tone := r.FormValue("tone")
			emoji := r.FormValue("emoji")
			body := strings.TrimSpace(r.FormValue("message"))
			if body == "" {
				message = "Klaxon message cannot be empty."
			} else if err := saveKlaxon(db, tone, emoji, body, time.Now()); err != nil {
				log.Errorf("Failed to save klaxon: %v", err)
				message = "Failed to save the klaxon."
			} else {
				success = "Klaxon updated."
			}
		}
	}

	klaxon, err := getKlaxon(db)
	if err != nil {
		log.Errorf("Failed to load klaxon: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Klaxon Unavailable", "We couldn't load the klaxon settings.", "/")
		return
	}

	authData := getAuthViewData(r)
	data := KlaxonAdminViewData{
		AuthViewData: authData,
		Klaxon:       klaxon,
		Error:        message,
		Success:      success,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "mod_klaxon.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func resolveReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
		return
	}
	if !requireModerator(w, r) {
		return
	}
	vars := mux.Vars(r)
	reportID, err := strconv.Atoi(vars["reportID"])
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Report", "That report ID is not valid.", "/")
		return
	}
	if err := r.ParseForm(); err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that resolution.", "/mod/reports")
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	username, _ := getAuthenticatedUsername(r)
	if err := resolveReport(db, reportID, username, note); err != nil {
		log.Errorf("Failed to resolve report: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Resolve Failed", "We couldn't resolve that report.", "/mod/reports")
		return
	}
	http.Redirect(w, r, "/mod/reports", http.StatusSeeOther)
}

func deletePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
		return
	}
	if !requireModerator(w, r) {
		return
	}
	vars := mux.Vars(r)
	postID, err := strconv.Atoi(vars["postID"])
	if err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Post", "That post ID is not valid.", "/")
		return
	}
	if err := r.ParseForm(); err != nil {
		renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that deletion.", "/")
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		renderErrorPage(w, r, http.StatusBadRequest, "Missing Reason", "Please add a reason for the removal.", "/")
		return
	}
	username, _ := getAuthenticatedUsername(r)
	if err := softDeletePost(db, postID, username, reason); err != nil {
		log.Errorf("Failed to delete post: %v", err)
		renderErrorPage(w, r, http.StatusInternalServerError, "Delete Failed", "We couldn't remove that post.", "/")
		return
	}
	next := sanitizeNext(r.FormValue("next"))
	if next == "" {
		threadID, err := getPostThreadID(db, postID)
		if err == nil {
			next = fmt.Sprintf("/view/thread/%d", threadID)
		} else {
			next = "/"
		}
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

type cardTreePayload struct {
	Trees []cardTreePayloadTree `json:"trees"`
}

type cardTreePayloadTree struct {
	Title       string                `json:"title"`
	Description string                `json:"description"`
	IsPrimary   bool                  `json:"is_primary"`
	Nodes       []cardTreePayloadNode `json:"nodes"`
}

type cardTreePayloadNode struct {
	TempID       string                      `json:"temp_id"`
	ParentTempID *string                     `json:"parent_temp_id"`
	CardName     string                      `json:"card_name"`
	Position     int                         `json:"position"`
	Annotations  []cardTreePayloadAnnotation `json:"annotations"`
}

type cardTreePayloadAnnotation struct {
	Kind  string `json:"kind"`
	Body  string `json:"body"`
	Label string `json:"label"`
	Tags  string `json:"tags"`
}

func parseCardTreePayload(raw string) (*cardTreePayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var payload cardTreePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if len(payload.Trees) == 0 {
		return nil, nil
	}
	return &payload, nil
}

func applyCardTreePayload(db *sql.DB, scopeType string, scopeID int, username string, payload *cardTreePayload) error {
	if payload == nil || len(payload.Trees) == 0 {
		return nil
	}
	for _, tree := range payload.Trees {
		title := strings.TrimSpace(tree.Title)
		if title == "" {
			return fmt.Errorf("tree title is required")
		}
		description := strings.TrimSpace(tree.Description)
		cardTree, err := createCardTree(db, scopeType, scopeID, title, description, username, tree.IsPrimary)
		if err != nil {
			return err
		}

		if len(tree.Nodes) == 0 {
			continue
		}

		idMap := make(map[string]int)
		pending := append([]cardTreePayloadNode(nil), tree.Nodes...)
		for len(pending) > 0 {
			progressed := false
			remaining := pending[:0]
			for _, node := range pending {
				cardName := strings.TrimSpace(node.CardName)
				if cardName == "" {
					return fmt.Errorf("card name is required")
				}
				if strings.TrimSpace(node.TempID) == "" {
					return fmt.Errorf("node id is required")
				}
				var parentID *int
				if node.ParentTempID != nil && strings.TrimSpace(*node.ParentTempID) != "" {
					parentDBID, ok := idMap[*node.ParentTempID]
					if !ok {
						remaining = append(remaining, node)
						continue
					}
					parentID = &parentDBID
				}
				createdNode, err := createCardTreeNode(db, cardTree.ID, parentID, cardName, node.Position, username)
				if err != nil {
					return err
				}
				idMap[node.TempID] = createdNode.ID
				progressed = true

				for _, annotation := range node.Annotations {
					body := strings.TrimSpace(annotation.Body)
					label := strings.TrimSpace(annotation.Label)
					tags := strings.TrimSpace(annotation.Tags)
					if body == "" {
						continue
					}
					kind := strings.TrimSpace(annotation.Kind)
					if kind == "" {
						kind = "note"
					}
					if _, err := createCardTreeAnnotation(db, createdNode.ID, kind, body, label, tags, nil, username); err != nil {
						return err
					}
				}
			}
			if !progressed && len(remaining) > 0 {
				return fmt.Errorf("invalid tree payload")
			}
			pending = remaining
		}
	}
	return nil
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := getAuthenticatedUsername(r); ok {
		next := sanitizeNext(r.URL.Query().Get("next"))
		if next == "" {
			next = "/profile"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
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
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
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
		renderErrorPage(w, r, http.StatusInternalServerError, "Profile Unavailable", "We couldn't load your profile.", "/")
		return
	}
	threads, err := getThreadsByAuthor(db, username)
	if err != nil {
		renderErrorPage(w, r, http.StatusInternalServerError, "Threads Unavailable", "We couldn't load your threads.", "/profile")
		return
	}
	posts, err := getPostsByAuthor(db, username)
	if err != nil {
		renderErrorPage(w, r, http.StatusInternalServerError, "Comments Unavailable", "We couldn't load your comments.", "/profile")
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
			renderErrorPage(w, r, http.StatusBadRequest, "Invalid Form", "We couldn't read that form submission.", "/user")
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
		renderErrorPage(w, r, http.StatusNotFound, "User Not Found", "We couldn't find that user.", "/user")
		return
	}
	user, err := getUserByUsername(db, username)
	if err != nil {
		renderErrorPage(w, r, http.StatusNotFound, "User Not Found", "We couldn't find that user.", "/user")
		return
	}
	threads, err := getThreadsByAuthor(db, username)
	if err != nil {
		renderErrorPage(w, r, http.StatusInternalServerError, "Threads Unavailable", "We couldn't load this user's threads.", "/user")
		return
	}
	posts, err := getPostsByAuthor(db, username)
	if err != nil {
		renderErrorPage(w, r, http.StatusInternalServerError, "Comments Unavailable", "We couldn't load this user's comments.", "/user")
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

func serveSignup(w http.ResponseWriter, r *http.Request) {
	if _, ok := getAuthenticatedUsername(r); ok {
		next := sanitizeNext(r.URL.Query().Get("next"))
		if next == "" {
			next = "/profile"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
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
		renderErrorPage(w, r, http.StatusMethodNotAllowed, "Not Allowed", "That action isn't supported here.", "/")
	}
}

func renderErrorPage(w http.ResponseWriter, r *http.Request, status int, title, message, backURL string) {
	authData := getAuthViewData(r)
	data := ErrorViewData{
		AuthViewData: authData,
		Title:        title,
		Message:      message,
		BackURL:      backURL,
	}
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "error.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func requireModerator(w http.ResponseWriter, r *http.Request) bool {
	if !requireAuth(w, r) {
		return false
	}
	username, _ := getAuthenticatedUsername(r)
	if !isModerator(username) {
		renderErrorPage(w, r, http.StatusForbidden, "Forbidden", "You don't have access to that page.", "/")
		return false
	}
	return true
}

func isValidReportCategory(category string) bool {
	for _, item := range reportCategories {
		if category == item {
			return true
		}
	}
	return false
}
