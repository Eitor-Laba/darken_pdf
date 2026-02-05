# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS build

# build-base instala gcc, g++, make, etc. Essencial para CGO.
RUN apk add --no-cache \
    build-base \
    mupdf-dev \
    pkgconfig

WORKDIR /app

# Copie primeiro apenas os arquivos de dependência para aproveitar o cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilação com CGO ativa
ENV CGO_ENABLED=1
# -tags musl garante que ele entenda o ambiente alpine
RUN go build -tags musl -o pdf-invert .

# --- Stage de Runtime ---
FROM alpine:3.21

# Precisamos das bibliotecas dinâmicas do mupdf para rodar o binário
RUN apk add --no-cache \
    mupdf \
    ca-certificates

WORKDIR /app

# Copia o binário e a pasta public
COPY --from=build /app/pdf-invert /app/pdf-invert
# Opcional: verifique se a pasta public realmente existe para não dar erro no COPY
COPY --from=build /app/public /app/public

# Garante que o binário tenha permissão de execução
RUN chmod +x /app/pdf-invert

EXPOSE 8080

CMD ["/app/pdf-invert"]