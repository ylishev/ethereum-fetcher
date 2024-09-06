FROM golang:1.23.0 AS builder

ADD . /build/

WORKDIR /build

RUN go mod download && go mod verify

RUN go build -v -o ../lime-server .

FROM gcr.io/distroless/base

LABEL maintainer="Yuliyan Lishev <july81@gmail.com>"

# make migrations files available at runtime
COPY --from=builder /build/internal/store/pg/migrations /internal/store/pg/migrations

COPY --from=builder /lime-server /lime-server

EXPOSE $APP_PORT

CMD ["./lime-server"]
