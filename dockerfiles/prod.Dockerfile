# クライアントサイドのビルド
FROM node:22.2-alpine3.18 as client-build

WORKDIR /app

COPY client/package*.json .
RUN npm ci

COPY client/ .

RUN npm run build


# サーバーサイドのビルド
FROM golang:1.22.3-alpine3.18 as server-build

WORKDIR /github.com/traP-jp/h23s_15

COPY server/go.mod .
COPY server/go.sum .
RUN go mod download

COPY server/ .

RUN go build -o app .


# 最終的な配信用
FROM alpine:3.20.3

WORKDIR /

RUN apk add --update --no-cache ca-certificates tzdata && update-ca-certificates \
    && cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime \
    && rm -rf /usr/share/zoneinfo

COPY --from=client-build /app/dist dist
COPY --from=server-build /github.com/traP-jp/h23s_15/app app

EXPOSE 8080
EXPOSE 8100

ENTRYPOINT [ "./app" ]