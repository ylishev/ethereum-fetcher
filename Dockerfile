FROM golang:1.23.0 AS builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download && go mod verify

ADD . ./

RUN go build -v -o ../lime-server .

FROM gcr.io/distroless/base

LABEL maintainer="Yuliyan Lishev <july81@gmail.com>"

# make migrations files available at runtime
COPY --from=builder /build/internal/store/pg/migrations /internal/store/pg/migrations

COPY --from=builder /lime-server /lime-server

# set default port if not specified during build
ARG API_PORT=8080
ENV API_PORT=${API_PORT}

EXPOSE ${API_PORT}

CMD ["./lime-server"]
