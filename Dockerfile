# Image of mansplainetes-webhook. The Go build runs on the build platform
# and cross-compiles, so a multi-arch build needs no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags="-s -w" -o /out/mansplainetes-webhook ./cmd/mansplainetes-webhook

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mansplainetes-webhook /mansplainetes-webhook
USER 65532:65532
ENTRYPOINT ["/mansplainetes-webhook"]
