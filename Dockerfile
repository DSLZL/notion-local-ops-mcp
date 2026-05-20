FROM golang:1.25 AS build

WORKDIR /src

COPY go.work main.go ./
COPY go ./go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/notion-local-ops-mcp ./main.go

FROM kalilinux/kali-rolling

ENV DEBIAN_FRONTEND=noninteractive
ARG TEMURIN8_API_URL="https://api.adoptium.net/v3/binary/latest/8/ga/linux/x64/jdk/hotspot/normal/eclipse"

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    kali-linux-headless \
    python3 python3-pip python3-venv \
    git curl wget jq file unzip xxd \
    nmap netcat-traditional socat \
    gdb strace ltrace \
    binutils checksec \
    python3-pwntools \
    sqlmap ffuf gobuster \
    openjdk-21-jdk-headless \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /opt/java \
  && curl -fsSL "${TEMURIN8_API_URL}" -o /tmp/temurin8.tar.gz \
  && tar -xzf /tmp/temurin8.tar.gz -C /opt/java \
  && rm -f /tmp/temurin8.tar.gz \
  && mv /opt/java/jdk8* /opt/java/jdk8 \
  && groupadd --system app \
  && useradd --system --gid app --home-dir /tmp --no-create-home app \
  && mkdir -p /tmp/notion-local-ops-mcp \
  && ln -sf /opt/java/jdk8/bin/java /usr/local/bin/java8 \
  && ln -sf /opt/java/jdk8/bin/javac /usr/local/bin/javac8 \
  && ln -sf /usr/lib/jvm/java-21-openjdk-amd64/bin/java /usr/local/bin/java21 \
  && ln -sf /usr/lib/jvm/java-21-openjdk-amd64/bin/javac /usr/local/bin/javac21 \
  && ln -sf /opt/java/jdk8/bin/java /usr/local/bin/java \
  && ln -sf /opt/java/jdk8/bin/javac /usr/local/bin/javac \
  && chown -R app:app /tmp

WORKDIR /tmp

COPY --from=build /out/notion-local-ops-mcp /usr/local/bin/notion-local-ops-mcp

ENV HOME=/tmp \
    JAVA8_HOME=/opt/java/jdk8 \
    JAVA21_HOME=/usr/lib/jvm/java-21-openjdk-amd64 \
    JAVA_HOME=/opt/java/jdk8 \
    NOTION_LOCAL_OPS_HOST=0.0.0.0 \
    NOTION_LOCAL_OPS_PORT=8766 \
    NOTION_LOCAL_OPS_WORKSPACE_ROOT=/tmp \
    NOTION_LOCAL_OPS_STATE_DIR=/tmp/notion-local-ops-mcp \
    NOTION_LOCAL_OPS_COMMAND_TIMEOUT=120

USER app

EXPOSE 8766

ENTRYPOINT ["/usr/local/bin/notion-local-ops-mcp"]
