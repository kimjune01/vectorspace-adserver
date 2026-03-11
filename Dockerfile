# Stage 1: Build Go server
FROM golang:1.25-alpine AS server-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG GIT_HASH=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.GitHash=${GIT_HASH}" -o /vectorspace-server ./cmd/server/

# Stage 2: Build portal frontend
FROM node:22-alpine AS portal-builder
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app/portal
COPY portal/package.json portal/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY portal/ .
ARG VITE_API_URL=""
RUN pnpm exec vite build

# Stage 3: Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=server-builder /vectorspace-server .
COPY --from=portal-builder /app/portal/dist ./portal-dist

EXPOSE 8080
VOLUME /data

ENTRYPOINT ["./vectorspace-server"]
CMD ["-db-path=/data/vectorspace.db", "-seed"]
