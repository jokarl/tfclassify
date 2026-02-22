FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

# Copy go.work and module files first for layer caching
COPY go.work go.work
COPY go.mod go.sum ./
COPY sdk/go.mod sdk/go.sum ./sdk/
COPY plugins/azurerm/go.mod plugins/azurerm/go.sum ./plugins/azurerm/

RUN go mod download

# Copy full source
COPY . .

# Build CLI and plugin with static linking
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tfclassify ./cmd/tfclassify
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tfclassify-plugin-azurerm ./plugins/azurerm

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/tfclassify /usr/local/bin/tfclassify
COPY --from=builder /out/tfclassify-plugin-azurerm /usr/local/bin/tfclassify-plugin-azurerm

ENTRYPOINT ["tfclassify"]
