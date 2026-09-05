FROM golang:1.27.1-alpine AS build
ARG VERSION=0.1.0-dev
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/atlasmesh ./cmd/atlasmesh

FROM scratch
COPY --from=build /out/atlasmesh /atlasmesh
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/atlasmesh"]
CMD ["server", "--listen", ":8080"]
