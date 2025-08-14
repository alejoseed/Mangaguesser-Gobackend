FROM golang:1.25-alpine

WORKDIR /app
COPY . .

RUN go mod download
RUN go get github.com/gin-contrib/cors
RUN go get github.com/google/uuid
RUN CGO_ENABLED=0 GOOS=linux go build -o /mangaguesser-app


FROM gcr.io/distroless/static
COPY --from=0 /mangaguesser-app /mangaguesser-app
COPY .env .
COPY mangaIDs.csv .

ENTRYPOINT ["/mangaguesser-app"]
