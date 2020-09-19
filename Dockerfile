FROM golang:1.15.2-buster AS builder
RUN apt-get update && apt-get install -y curl unzip upx

ENV PROTO_VERSION 3.13.0

RUN PROTOC_ZIP="protoc-${PROTO_VERSION}-linux-x86_64.zip" && \
    curl -sOL "https://github.com/google/protobuf/releases/download/v${PROTO_VERSION}/${PROTOC_ZIP}" && \
    unzip -o ${PROTOC_ZIP} -d /usr/local bin/protoc && \
    unzip -o ${PROTOC_ZIP} -d /usr/local 'include/*' && \
    rm -f ${PROTOC_ZIP}
RUN protoc --version
WORKDIR /workspace
# This will allow caching dependencies and not triggering
# fetching dependencies for every code change
COPY go.* ./
RUN go mod download

COPY protoc-code.sh .
COPY pb pb
RUN ./protoc-code.sh

COPY . /workspace
RUN ./protoc-doc.sh
ENV CGO_ENABLED 0
RUN go build -ldflags "-s -w" \
	-mod=readonly -o bin/service cmd/server/main.go
# Compress the binary
RUN upx bin/service

ENV GRPC_HEALTH_PROBE_VERSION 0.3.2
RUN curl -sL "https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64" \
    -o grpc-health && chmod +x grpc-health

FROM scratch
COPY --from=builder /workspace/grpc-health /bin/grpc-health
COPY --from=builder /workspace/bin/service /bin/service
# Content server from HTTP paths /docs and /proto
COPY --from=builder /workspace/pb/service.proto /static/service.proto
COPY --from=builder /workspace/pb/index.html /static/index.html
USER 1000
ENTRYPOINT [ "/bin/service" ]
