# ----------------------------------
# 1) Build Stage
# ----------------------------------
FROM golang:1.23-alpine AS build

# Create and set the working directory
WORKDIR /app

# Copy go.mod and go.sum first for dependency download caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application files
COPY . .

# Build the Go application
RUN go build -o server main.go

# ----------------------------------
# 2) Runtime Stage
# ----------------------------------
FROM alpine:3.18

# Create an app directory
WORKDIR /app

# Copy the compiled binary from the build stage
COPY --from=build /app/server /app/

# Copy templates folder (embedded resources may still require the folder structure)
COPY --from=build /app/templates ./templates

# Expose port 8080 for the application
EXPOSE 8080

# Run the server binary
ENTRYPOINT ["/app/server"]

