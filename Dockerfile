FROM node:24-alpine AS webbuild
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webbuild /src/web/dist ./web/dist
RUN go build -o /out/finnews ./cmd/finnews

FROM alpine:3.22
WORKDIR /app
RUN adduser -D appuser
COPY --from=gobuild /out/finnews /usr/local/bin/finnews
COPY config ./config
COPY db ./db
COPY prompts ./prompts
USER appuser
EXPOSE 8080
CMD ["finnews", "server"]
