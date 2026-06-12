FROM golang:1.25-alpine AS build

WORKDIR /src

COPY basis/go.mod basis/go.sum ./
RUN go mod download

COPY basis/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/celine
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker


FROM gcr.io/distroless/static-debian12 AS server
COPY --from=build /out/server /server
ENTRYPOINT ["/server"]


FROM gcr.io/distroless/static-debian12 AS worker
COPY --from=build /out/worker /worker
ENTRYPOINT ["/worker"]
