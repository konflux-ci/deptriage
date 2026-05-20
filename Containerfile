FROM registry.access.redhat.com/ubi10/go-toolset:10.2-1777890685 AS builder

USER root
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /deptriage ./cmd/deptriage/

FROM registry.access.redhat.com/ubi10/go-toolset:10.2-1777890685 AS tools

USER root
ENV GOPATH=/tmp/go
RUN go install golang.org/x/vuln/cmd/govulncheck@latest

FROM registry.access.redhat.com/ubi10/ubi-minimal:10.2-1777462752

COPY --from=builder /deptriage /usr/local/bin/deptriage
COPY --from=tools /tmp/go/bin/govulncheck /usr/local/bin/govulncheck

USER 1001

ENTRYPOINT ["deptriage"]
