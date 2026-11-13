FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
# go mod tidy resolves go.sum on first build; commit the generated go.sum for reproducible builds
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server .

FROM alpine:3.20
RUN adduser -D app && apk add --no-cache ca-certificates tzdata
COPY --from=build /out/server /server
USER app
EXPOSE 8080
CMD ["/server"]
