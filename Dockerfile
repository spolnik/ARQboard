FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS server
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/arqboard ./cmd/arqboard

FROM alpine:3.20
RUN addgroup -S arqboard && adduser -S -G arqboard arqboard
WORKDIR /app
COPY --from=server /out/arqboard /usr/local/bin/arqboard
COPY --from=web /src/web/dist /app/web/dist
RUN mkdir -p /app/data && chown -R arqboard:arqboard /app
USER arqboard
ENV HTTP_ADDR=:8080
ENV WEB_DIST_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["arqboard"]
CMD ["serve"]
