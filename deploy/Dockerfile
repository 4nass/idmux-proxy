FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/idmux-proxy ./cmd/idmux-proxy

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/idmux-proxy /idmux-proxy
USER nonroot:nonroot
EXPOSE 9000
ENTRYPOINT ["/idmux-proxy"]
