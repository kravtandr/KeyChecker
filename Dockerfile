# --- frontend build ---
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- backend build ---
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./internal/httpapi/webdist
RUN CGO_ENABLED=0 go build -o /out/keychecker ./cmd/keychecker

# --- runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/keychecker /keychecker
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/keychecker"]
