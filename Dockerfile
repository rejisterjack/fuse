# syntax=docker/dockerfile:1

FROM golang:1.27.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fuse ./cmd/fuse

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/fuse /fuse
COPY config.example.yaml /config.example.yaml
EXPOSE 8080
ENTRYPOINT ["/fuse"]
CMD ["-config", "/config.example.yaml"]
