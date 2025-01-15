#!/bin/bash

# Base URL
BASE_URL="http://localhost:8080"

# Create a board
echo "Creating a board..."
curl -X POST -H "Content-Type: application/json" -d '{"name":"/salt/", "description":"let the hate flow"}' $BASE_URL/boards
echo -e "\n"

# List boards
echo "Listing boards..."
curl $BASE_URL/boards
echo -e "\n"

# Create a thread in the board
echo "Creating a thread in board 1..."
curl -X POST -H "Content-Type: application/json" -d '{"title":"Dimir control is OP"}' $BASE_URL/threads/1
echo -e "\n"

# List threads for the board
echo "Listing threads for board 1..."
curl $BASE_URL/threads/1
echo -e "\n"

# Create a post in the thread
echo "Creating a post in thread 1..."
curl -X POST -H "Content-Type: application/json" -d '{"author":"anonymous", "content":"bofades nutz"}' $BASE_URL/posts/1/1
echo -e "\n"

# List posts for the thread
echo "Listing posts for thread 1..."
curl $BASE_URL/threads/1
echo -e "\n"

# Fetch a specific board
echo "Fetching board 1..."
curl $BASE_URL/boards/1
echo -e "\n"

# Fetch a specific thread
echo "Fetching thread 1..."
curl $BASE_URL/threads/1
echo -e "\n"
