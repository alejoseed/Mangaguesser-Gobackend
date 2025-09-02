FROM golang:1.25.0-alpine3.22 AS build

ENV GO111MODULE=on \
    GOPROXY=https://proxy.golang.org,direct \
    CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -ldflags="-s -w" -o /mangaguesser-app

FROM gcr.io/distroless/static
WORKDIR /app

COPY --from=build /mangaguesser-app /mangaguesser-app
COPY .env .
COPY mangaIDs.csv .

ENTRYPOINT ["/mangaguesser-app"]
