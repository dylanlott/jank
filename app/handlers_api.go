package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// ------------------- REST Handlers (JSON) -------------------

type treeCreateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	IsPrimary   bool   `json:"is_primary"`
}

type nodeCreateRequest struct {
	ParentID *int   `json:"parent_id"`
	CardName string `json:"card_name"`
	Position int    `json:"position"`
}

type nodeUpdateRequest struct {
	ParentID *int   `json:"parent_id"`
	CardName string `json:"card_name"`
	Position int    `json:"position"`
}

type annotationCreateRequest struct {
	Kind         string `json:"kind"`
	Body         string `json:"body"`
	Label        string `json:"label"`
	Tags         string `json:"tags"`
	SourcePostID *int   `json:"source_post_id"`
}

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

// boardTreesHandler lists or creates trees under a board (REST API).
func boardTreesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardIDStr := vars["boardID"]
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		http.Error(w, "Invalid Board ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		trees, err := getCardTreesByScope(db, "board", boardID, false)
		if err != nil {
			log.Errorf("Failed to retrieve board trees: %v", err)
			http.Error(w, "Failed to retrieve trees", http.StatusInternalServerError)
			return
		}
		respondJSON(w, trees)

	case http.MethodPost:
		if !requireAPIAuth(w, r) {
			return
		}
		username, _ := getBearerUsername(r)
		var req treeCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "Title is required", http.StatusBadRequest)
			return
		}
		tree, err := createCardTree(db, "board", boardID, req.Title, req.Description, username, req.IsPrimary)
		if err != nil {
			log.Errorf("Failed to create board tree: %v", err)
			http.Error(w, "Failed to create tree", http.StatusInternalServerError)
			return
		}
		respondJSON(w, tree)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// threadTreesHandler lists or creates trees under a thread (REST API).
func threadTreesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadIDStr := vars["threadID"]
	threadID, err := strconv.Atoi(threadIDStr)
	if err != nil {
		http.Error(w, "Invalid Thread ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		trees, err := getCardTreesByScope(db, "thread", threadID, false)
		if err != nil {
			log.Errorf("Failed to retrieve thread trees: %v", err)
			http.Error(w, "Failed to retrieve trees", http.StatusInternalServerError)
			return
		}
		respondJSON(w, trees)

	case http.MethodPost:
		if !requireAPIAuth(w, r) {
			return
		}
		username, _ := getBearerUsername(r)
		var req treeCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "Title is required", http.StatusBadRequest)
			return
		}
		tree, err := createCardTree(db, "thread", threadID, req.Title, req.Description, username, req.IsPrimary)
		if err != nil {
			log.Errorf("Failed to create thread tree: %v", err)
			http.Error(w, "Failed to create tree", http.StatusInternalServerError)
			return
		}
		respondJSON(w, tree)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// treeHandler fetches a specific tree with nodes and annotations (REST API).
func treeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	treeIDStr := vars["treeID"]
	treeID, err := strconv.Atoi(treeIDStr)
	if err != nil {
		http.Error(w, "Invalid Tree ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tree, err := getCardTreeByID(db, treeID)
	if err != nil {
		log.Errorf("Tree not found: %v", err)
		http.Error(w, "Tree not found", http.StatusNotFound)
		return
	}
	respondJSON(w, tree)
}

// treeNodesHandler creates nodes under a tree (REST API).
func treeNodesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	treeIDStr := vars["treeID"]
	treeID, err := strconv.Atoi(treeIDStr)
	if err != nil {
		http.Error(w, "Invalid Tree ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAPIAuth(w, r) {
		return
	}
	username, _ := getBearerUsername(r)
	var req nodeCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.CardName == "" {
		http.Error(w, "Card name is required", http.StatusBadRequest)
		return
	}
	node, err := createCardTreeNode(db, treeID, req.ParentID, req.CardName, req.Position, username)
	if err != nil {
		log.Errorf("Failed to create tree node: %v", err)
		http.Error(w, "Failed to create node", http.StatusInternalServerError)
		return
	}
	respondJSON(w, node)
}

// treeNodeHandler updates or deletes a tree node (REST API).
func treeNodeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	treeIDStr := vars["treeID"]
	nodeIDStr := vars["nodeID"]
	treeID, err := strconv.Atoi(treeIDStr)
	if err != nil {
		http.Error(w, "Invalid Tree ID", http.StatusBadRequest)
		return
	}
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		http.Error(w, "Invalid Node ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		if !requireAPIAuth(w, r) {
			return
		}
		var req nodeUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.CardName == "" {
			http.Error(w, "Card name is required", http.StatusBadRequest)
			return
		}
		nodeTreeID, err := getCardTreeNodeTreeID(db, nodeID)
		if err != nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		if nodeTreeID != treeID {
			http.Error(w, "Node does not belong to tree", http.StatusBadRequest)
			return
		}
		if err := updateCardTreeNode(db, nodeID, req.ParentID, req.CardName, req.Position); err != nil {
			log.Errorf("Failed to update tree node: %v", err)
			http.Error(w, "Failed to update node", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		if !requireAPIAuth(w, r) {
			return
		}
		nodeTreeID, err := getCardTreeNodeTreeID(db, nodeID)
		if err != nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		if nodeTreeID != treeID {
			http.Error(w, "Node does not belong to tree", http.StatusBadRequest)
			return
		}
		if err := deleteCardTreeNode(db, nodeID); err != nil {
			log.Errorf("Failed to delete tree node: %v", err)
			http.Error(w, "Failed to delete node", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// treeNodeAnnotationsHandler creates annotations for a tree node (REST API).
func treeNodeAnnotationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	treeIDStr := vars["treeID"]
	nodeIDStr := vars["nodeID"]
	treeID, err := strconv.Atoi(treeIDStr)
	if err != nil {
		http.Error(w, "Invalid Tree ID", http.StatusBadRequest)
		return
	}
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		http.Error(w, "Invalid Node ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAPIAuth(w, r) {
		return
	}
	nodeTreeID, err := getCardTreeNodeTreeID(db, nodeID)
	if err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	if nodeTreeID != treeID {
		http.Error(w, "Node does not belong to tree", http.StatusBadRequest)
		return
	}
	username, _ := getBearerUsername(r)
	var req annotationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "note"
	}
	if req.Body == "" {
		http.Error(w, "Body is required", http.StatusBadRequest)
		return
	}
	annotation, err := createCardTreeAnnotation(db, nodeID, kind, req.Body, req.Label, req.Tags, req.SourcePostID, username)
	if err != nil {
		log.Errorf("Failed to create annotation: %v", err)
		http.Error(w, "Failed to create annotation", http.StatusInternalServerError)
		return
	}
	respondJSON(w, annotation)
}

// treeNodeAnnotationHandler deletes an annotation (REST API).
func treeNodeAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	annotationIDStr := vars["annotationID"]
	annotationID, err := strconv.Atoi(annotationIDStr)
	if err != nil {
		http.Error(w, "Invalid Annotation ID", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAPIAuth(w, r) {
		return
	}
	if err := deleteCardTreeAnnotation(db, annotationID); err != nil {
		log.Errorf("Failed to delete annotation: %v", err)
		http.Error(w, "Failed to delete annotation", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
