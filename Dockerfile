ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    procps \
    npm \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY package.json package-lock.json tailwind.config.js Makefile ./
RUN npm install

COPY templates ./templates
COPY assets ./assets
COPY static ./static

RUN make css

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN make app

CMD ["/app/dnssecmenot"]
