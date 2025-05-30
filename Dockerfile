# Use the official Go image as a build stage
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o milo ./cmd/milo

# Use a minimal image for the final container
FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /app/milo .
ENTRYPOINT ["/milo"]
