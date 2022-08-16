FROM golang:1.19-bullseye as build

WORKDIR /go/app
COPY go.mod go.sum ./
RUN go mod download
COPY . /go/app/
RUN go build -o prom-dns-http-sd ./cmd/app/main.go


FROM debian:bullseye

RUN apt update \
    && apt install -y ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /go/app/prom-dns-http-sd /usr/local/bin/prom-dns-http-sd

USER 1000:1000

EXPOSE 8080/tcp

ENTRYPOINT ["/usr/local/bin/prom-dns-http-sd"]
