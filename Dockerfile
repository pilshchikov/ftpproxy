FROM golang:1.16-alpine as backend

ADD . /build
WORKDIR /build/app

RUN go get && go build -o /build/ftpproxy


FROM alpine
COPY --from=backend /build/ftpproxy /srv/ftpproxy

WORKDIR /srv
ENTRYPOINT /srv/ftpproxy

