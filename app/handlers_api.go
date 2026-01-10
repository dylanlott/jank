package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

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
