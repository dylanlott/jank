# jank

```txt
       _             __  
      (_)___ _____  / /__
     / / __ `/ __ \/ //_/
    / / /_/ / / / / ,<   
 __/ /\__,_/_/ /_/_/|_|  
/___/                    
```

> a tcg-focused forum

`jank` uses Go with PostgreSQL by default (SQLite is optional) to store data. It is intentionally simple with all front-end assets statically embedded in the Go binary at build time. The server listens on `http://localhost:8080`.

## Development

### Quick start (SQLite)

The Makefile defaults to SQLite for local development:

```sh
make run
```

Or run directly:

```sh
export JANK_DB_DRIVER="sqlite"
export JANK_DB_DSN="./sqlite.db"
go run .
```

To override the HTTP listen address, set `JANK_ADDR` (full `host:port`) or `JANK_PORT` / `PORT` (port only).

### PostgreSQL

If you want Postgres (the default when `JANK_DB_DRIVER` is unset), set the DSN:

```sh
export JANK_DB_DSN="postgres://user:pass@localhost:5432/jank?sslmode=disable"
go run .
```

You can also set `DATABASE_URL` instead of `JANK_DB_DSN`.

### Auth config

Posting threads or comments via HTML views requires a login cookie. Configure credentials with:

```sh
export JANK_FORUM_USER="admin"
export JANK_FORUM_PASS="admin"
export JANK_FORUM_SECRET="change-me"
export JANK_JWT_SECRET="change-me-too"
```

If secrets are omitted, they are generated per process (see logs). You can also sign up via `/signup` to create additional users.

### Announcements (klaxon banner)

Moderators can set the site-wide klaxon banner from `/mod/klaxon`. The data is persisted in the database and renders across all pages.

### Search (boards + threads + posts)

The `/search` page queries board names/descriptions and thread titles/tags/authors, plus post content. SQLite uses FTS5 with prefix matching when available, and falls back to `LIKE` if FTS5 is not compiled in.

For SQLite, migrations create the following FTS tables and triggers and rebuild them on startup:

- `boards_fts`
- `threads_fts`
- `posts_fts`

If your SQLite build lacks FTS5 (you see `no such module: fts5`), search still works but without fuzzy ranking. To enable FTS5 with `mattn/go-sqlite3`, build/run with:

```sh
go run -tags sqlite_fts5 main.go
```

PostgreSQL does not use FTS tables here; search falls back to `ILIKE` queries against boards, threads, and post content.

To force a rebuild of SQLite search indexes without restarting the app, run:

```sh
sqlite3 ./sqlite.db "INSERT INTO boards_fts(boards_fts) VALUES('rebuild'); INSERT INTO threads_fts(threads_fts) VALUES('rebuild'); INSERT INTO posts_fts(posts_fts) VALUES('rebuild');"
```

### JSON API authentication (JWT)

Creating or deleting boards, and creating threads or posts via the JSON API requires a JWT in the `Authorization` header.

Issue a token:

```sh
curl -X POST -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  http://localhost:8080/auth/token
```

Create a user and receive a token:

```sh
curl -X POST -H "Content-Type: application/json" \
  -d '{"username":"newuser","password":"changeme"}' \
  http://localhost:8080/auth/signup
```

Use the token:

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title":"Dimir control is OP"}' \
  http://localhost:8080/threads/2
```

## Moderation

Moderation is tied to the forum admin user (`JANK_FORUM_USER`). That username is treated as the moderator for both HTML and API flows.

HTML endpoints (cookie auth, moderator only):

- `GET /mod/reports` moderation queue
- `POST /mod/reports/{reportID}/resolve` resolve a report (`note` form field)
- `POST /mod/posts/{postID}/delete` soft-delete a post (`reason`, optional `next`)

JSON API endpoints (JWT auth; moderator required unless noted):

- `POST /reports` create a report (any authenticated user)
- `GET /reports` list open reports (moderator)
- `POST /reports/{reportID}/resolve` resolve a report (moderator)
- `POST /posts/{postID}/delete` soft-delete a post (moderator)

Example: create and resolve a report

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"post_id":1,"category":"spam","reason":"off-topic"}' \
  http://localhost:8080/reports

curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"note":"removed"}' \
  http://localhost:8080/reports/1/resolve
```

## API smoke tests

Most write endpoints require a JWT; see "JSON API authentication" above.

### Create a board

```sh
curl -X POST -H "Content-Type: application/json" -d '{"name":"/salt/", "description":"let the hate flow"}' http://localhost:8080/boards
```

### List boards

```sh
curl http://localhost:8080/boards
```

### List threads for a given board

```sh
curl http://localhost:8080/threads/1
```

### Create a post in a thread

```sh
curl -X POST -H "Content-Type: application/json" -d '{"title":"Dimir control is OP"}' http://localhost:8080/threads/2
```

### Create post in a thread

```sh
curl -X POST -H "Content-Type: application/json" -d '{"author":"anonymous", "content":"bofades nutz"}' http://localhost:8080/posts/1/1
```

### Create a card tree for a board

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title":"Azorius Control Core","description":"Primary shells","is_primary":true}' \
  http://localhost:8080/boards/1/trees
```

### List card trees for a board

```sh
curl http://localhost:8080/boards/1/trees
```

### Create a card tree for a thread

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title":"Mirror Sideboard Map","description":"Matchup plan","is_primary":false}' \
  http://localhost:8080/threads/2/trees
```

### Fetch a tree with nodes and annotations

```sh
curl http://localhost:8080/trees/1
```

### Add a node to a tree

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"card_name":"Teferi, Hero of Dominaria","parent_id":null,"position":0}' \
  http://localhost:8080/trees/1/nodes
```

### Update a node in a tree

```sh
curl -X PATCH -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"card_name":"Teferi, Time Raveler","parent_id":null,"position":1}' \
  http://localhost:8080/trees/1/nodes/1
```

### Delete a node in a tree

```sh
curl -X DELETE -H "Authorization: Bearer <token>" http://localhost:8080/trees/1/nodes/1
```

### Add an annotation to a node

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"kind":"note","body":"Pairs with [[Narset, Parter of Veils]]","source_post_id":null}' \
  http://localhost:8080/trees/1/nodes/1/annotations
```

### Delete an annotation

```sh
curl -X DELETE -H "Authorization: Bearer <token>" http://localhost:8080/trees/1/nodes/1/annotations/1
```

## Tooling

Common Make targets:

- `make run` (SQLite), `make dev` (hot reload via `air`), `make test`, `make fmt`, `make tidy`, `make build`, `make backup-db`

## Ideas

- Ability to represent annotated trees of cards
- Decklist handling
- Player profile integration with vedh.xyz
- Replay and game linking support for vedh games

## License

This project is licensed under the MIT License. See the [MIT LICENSE](./LICENSE) file for details.
