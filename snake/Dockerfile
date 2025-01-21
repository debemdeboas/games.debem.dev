FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY snake/ ./snake/
RUN go build -ldflags="-s -w" -v -o /usr/local/bin/app ./snake/cmd/ssh

CMD ["app"]
