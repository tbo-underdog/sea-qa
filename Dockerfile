# build
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/sea-qa ./cmd/sea-qa

# runtime
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/sea-qa /usr/local/bin/sea-qa
ENTRYPOINT ["sea-qa"]
