# jank

> a messaging board with MTG card recognition for vedh.xyz

## Stack

This app uses Go with SQLite to store data. It is intentionally thin with all front-end assets statically embedded in the Go binary at build time.

## Development

To run the server:

```sh
go run main.go
```

## Testing

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
