###### build stage ####
FROM golang:1.17-stretch as build

WORKDIR /app

RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.23.3/bin/linux/$(uname -m | sed -E 's/x86_64/amd64/g' | sed -E 's/aarch64/arm64/g')/kubectl -O /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl

ADD go.mod .
ADD go.sum .

RUN go mod download

ADD . .

RUN go build -ldflags "-s -w" -o ./dist/cacheproxy ./cmd/cacheproxy/main.go && \
    go build -ldflags "-s -w" -o ./dist/kubectl-ckube ./cmd/ckube-plugin/main.go

FROM ubuntu:20.04

WORKDIR /app

COPY --from=build /usr/bin/kubectl /usr/local/bin/
COPY --from=build /app/dist/kubectl-ckube /usr/local/bin/
COPY --from=build /app/dist/cacheproxy dist/cacheproxy

CMD ["/app/dist/cacheproxy"]
