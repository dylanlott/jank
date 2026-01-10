# jank

```txt
       _             __  
      (_)___ _____  / /__
     / / __ `/ __ \/ //_/
    / / /_/ / / / / ,<   
 __/ /\__,_/_/ /_/_/|_|  
/___/                    
```

`jank` uses Go with SQLite to store data. It is intentionally simple with all front-end assets statically embedded in the Go binary at build time.

## Development

To run the server:

```sh
go run main.go
```

### Forum authentication (HTML views)

Posting threads or comments via the HTML views requires a login cookie. Configure credentials with:

```sh
export JANK_FORUM_USER="admin"
export JANK_FORUM_PASS="admin"
export JANK_FORUM_SECRET="change-me"
export JANK_JWT_SECRET="change-me-too"
```

You can also sign up via `/signup` to create additional users.

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

## Ideas

- Ability to represent annotated tress of cards
- Decklist handling
- Player profile integration with vedh.xyz
- Replay and game linking support for vedh games

## License

This project is licensed under the MIT License. See the [MIT LICENSE](./LICENSE) file for details.
