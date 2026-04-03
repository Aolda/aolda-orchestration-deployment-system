FROM golang:1.24.6-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/aods-backend ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY --from=build /out/aods-backend /usr/local/bin/aods-backend
COPY entrypoint.sh /usr/local/bin/aods-backend-entrypoint.sh

RUN chmod +x /usr/local/bin/aods-backend-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/aods-backend-entrypoint.sh"]
