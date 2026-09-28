# ═══════════ STAGE 1: BUILD ═══════════
FROM golang:1.24-alpine AS builder
WORKDIR /src

# copy go.mod ก่อน เพื่อใช้ layer cache
COPY go.mod ./
RUN go mod download

COPY main.go ./

ARG VERSION=0.1.0
# CGO_ENABLED=0 → static binary ไม่ต้องพึ่ง libc
# -s -w         → ตัด debug symbol ให้เล็กลง
# -trimpath     → ไม่ฝัง path ของเครื่อง build
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/app .

# ═══════════ STAGE 2: RUNTIME ═══════════
# distroless = ไม่มี shell ไม่มี package manager
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app"]
