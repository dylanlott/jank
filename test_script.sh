#!/bin/bash

# Base URL
BASE_URL="http://localhost:8080"
AUTH_USER="${AUTH_USER:-admin}"
AUTH_PASS="${AUTH_PASS:-admin}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to run this script."
  exit 1
fi

echo "Requesting auth token..."
TOKEN=$(curl -s -X POST -H "Content-Type: application/json" \
  -d "{\"username\":\"${AUTH_USER}\",\"password\":\"${AUTH_PASS}\"}" \
  "$BASE_URL/auth/token" | jq -r '.token')

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "Failed to obtain auth token."
  exit 1
fi

AUTH_HEADER="Authorization: Bearer $TOKEN"

# Create a board
echo "Creating a board..."
BOARD_ID=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"name":"/salt/", "description":"let the hate flow"}' \
  "$BASE_URL/boards" | jq -r '.id')
if [[ -z "$BOARD_ID" || "$BOARD_ID" == "null" ]]; then
  echo "Failed to create board."
  exit 1
fi
echo "Created board ID: $BOARD_ID"
echo -e "\n"

# List boards
echo "Listing boards..."
curl $BASE_URL/boards
echo -e "\n"

# Create a thread in the board
echo "Creating a thread in board $BOARD_ID..."
THREAD_ID=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"title":"Dimir control is OP"}' \
  "$BASE_URL/threads/$BOARD_ID" | jq -r '.id')
if [[ -z "$THREAD_ID" || "$THREAD_ID" == "null" ]]; then
  echo "Failed to create thread."
  exit 1
fi
echo "Created thread ID: $THREAD_ID"
echo -e "\n"

# List threads for the board
echo "Listing threads for board $BOARD_ID..."
curl "$BASE_URL/threads/$BOARD_ID"
echo -e "\n"

# Create a post in the thread
echo "Creating a post in thread $THREAD_ID..."
curl -X POST -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"author":"anonymous", "content":"bofades nutz"}' \
  "$BASE_URL/posts/$BOARD_ID/$THREAD_ID"
echo -e "\n"

# List threads for the board (posts are included in board fetch later)
echo "Listing threads for board $BOARD_ID..."
curl "$BASE_URL/threads/$BOARD_ID"
echo -e "\n"

# Fetch a specific board
echo "Fetching board $BOARD_ID..."
curl "$BASE_URL/boards/$BOARD_ID"
echo -e "\n"

# Fetch a specific thread
echo "Fetching threads for board $BOARD_ID..."
curl "$BASE_URL/threads/$BOARD_ID"
echo -e "\n"

# Test delete board route
echo "Testing DELETE /delete/board/{boardID} route..."

# Create a new board to delete
DELETE_BOARD_ID=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"name": "Test Board", "description": "This board will be deleted."}' \
  "$BASE_URL/boards" | jq -r '.id')
if [[ -z "$DELETE_BOARD_ID" || "$DELETE_BOARD_ID" == "null" ]]; then
  echo "Failed to create delete-test board."
  exit 1
fi
echo "Created board ID to delete: $DELETE_BOARD_ID"

# Delete the board
curl -X DELETE -H "$AUTH_HEADER" "$BASE_URL/delete/board/$DELETE_BOARD_ID"

# Verify the board was deleted
RESPONSE=$(curl -s "$BASE_URL/boards/$DELETE_BOARD_ID")
if [[ $RESPONSE == *"Board not found"* ]]; then
  echo "Board successfully deleted."
else
  echo "Failed to delete board."
fi
