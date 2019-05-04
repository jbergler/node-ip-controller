FROM golang as builder

WORKDIR /src

# Setup dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build

# final image
FROM alpine:latest
LABEL maintainer="Jonas Bergler <jonas@bergler.name>"

RUN apk update && \
    apk upgrade && \
    apk add --no-cache ca-certificates && \
    update-ca-certificates 2>/dev/null || true

COPY --from=builder /src/build/node-ip-controller /bin/node-ip-controller

USER nobody

ENTRYPOINT ["/bin/node-ip-controller"]
