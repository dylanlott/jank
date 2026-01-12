package app

import "github.com/gorilla/mux"

func buildRouter() *mux.Router {
	r := mux.NewRouter()

	// HTML pages
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
	r.HandleFunc("/favicon.ico", serveFaviconRedirect).Methods("GET")
	r.HandleFunc("/favicon.svg", serveFavicon).Methods("GET")
	r.HandleFunc("/auth/token", authTokenHandler).Methods("POST")
	r.HandleFunc("/auth/signup", authSignupHandler).Methods("POST")

	// REST API endpoints
	r.HandleFunc("/boards", boardsHandler).Methods("GET", "POST")
	r.HandleFunc("/boards/{boardID:[0-9]+}", boardHandler).Methods("GET")
	r.HandleFunc("/boards/{boardID:[0-9]+}/trees", boardTreesHandler).Methods("GET", "POST")
	r.HandleFunc("/threads/{boardID:[0-9]+}", threadsHandler).Methods("GET", "POST")
	r.HandleFunc("/threads/{threadID:[0-9]+}/trees", threadTreesHandler).Methods("GET", "POST")
	r.HandleFunc("/posts/{boardID:[0-9]+}/{threadID:[0-9]+}", postsHandler).Methods("POST")
	r.HandleFunc("/trees/{treeID:[0-9]+}", treeHandler).Methods("GET")
	r.HandleFunc("/trees/{treeID:[0-9]+}/nodes", treeNodesHandler).Methods("POST")
	r.HandleFunc("/trees/{treeID:[0-9]+}/nodes/{nodeID:[0-9]+}", treeNodeHandler).Methods("PATCH", "DELETE")
	r.HandleFunc("/trees/{treeID:[0-9]+}/nodes/{nodeID:[0-9]+}/annotations", treeNodeAnnotationsHandler).Methods("POST")
	r.HandleFunc("/trees/{treeID:[0-9]+}/nodes/{nodeID:[0-9]+}/annotations/{annotationID:[0-9]+}", treeNodeAnnotationHandler).Methods("DELETE")
	r.HandleFunc("/delete/board/{boardID:[0-9]+}", deleteBoardHandler).Methods("DELETE")

	return r
}
