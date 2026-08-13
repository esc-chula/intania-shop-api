FROM golang:1.26.5-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/intania-shop-api .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/intania-shop-api /intania-shop-api

EXPOSE 8080

ENTRYPOINT ["/intania-shop-api"]
CMD ["serve"]
