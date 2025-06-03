FROM golang:1.24.3

WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code.
COPY *.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /email-service

EXPOSE 8080

# Run
CMD ["/email-service"]