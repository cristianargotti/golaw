FROM golang:1.27.3-trixie@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN go build -o /out/hub ./cmd/hub

FROM gcr.io/distroless/static-debian13@sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
COPY --from=build /out/hub /hub
USER 65532:65532
ENTRYPOINT ["/hub"]
