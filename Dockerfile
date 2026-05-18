FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.work main.go ./
COPY go ./go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/notion-local-ops-mcp ./main.go

FROM alpine:3.22
RUN apk add --no-cache \
    python3 \
    py3-pip \
    openjdk8-jdk \
    openjdk21-jdk \
 && addgroup -S app \
 && adduser -S -G app app \
 && ln -sf /usr/lib/jvm/java-1.8-openjdk/bin/java /usr/local/bin/java8 \
 && ln -sf /usr/lib/jvm/java-1.8-openjdk/bin/javac /usr/local/bin/javac8 \
 && ln -sf /usr/lib/jvm/java-21-openjdk/bin/java /usr/local/bin/java21 \
 && ln -sf /usr/lib/jvm/java-21-openjdk/bin/javac /usr/local/bin/javac21 \
 && ln -sf /usr/lib/jvm/java-21-openjdk/bin/java /usr/local/bin/java \
 && ln -sf /usr/lib/jvm/java-21-openjdk/bin/javac /usr/local/bin/javac

WORKDIR /app

COPY --from=build /out/notion-local-ops-mcp /usr/local/bin/notion-local-ops-mcp
RUN mkdir -p /app/CTF && chown -R app:app /app

ENV HOME=/tmp \
    JAVA8_HOME=/usr/lib/jvm/java-1.8-openjdk \
    JAVA21_HOME=/usr/lib/jvm/java-21-openjdk \
    JAVA_HOME=/usr/lib/jvm/java-21-openjdk \
    NOTION_LOCAL_OPS_HOST=0.0.0.0 \
    NOTION_LOCAL_OPS_PORT=8766 \
    NOTION_LOCAL_OPS_WORKSPACE_ROOT=/app/CTF \
    NOTION_LOCAL_OPS_STATE_DIR=/tmp/notion-local-ops-mcp \
    NOTION_LOCAL_OPS_COMMAND_TIMEOUT=120

USER app

EXPOSE 8766

ENTRYPOINT ["/usr/local/bin/notion-local-ops-mcp"]
