###### build stage ####
FROM golang:1.17-stretch as build

WORKDIR /app

ADD go.mod .
ADD go.sum .

RUN go mod download

ADD . .

RUN go build -ldflags "-s -w" -o ./dist/cacheproxy cmd/cacheproxy/main.go


FROM nginx:alpine

WORKDIR /app

COPY --from=build /app/dist/cacheproxy dist/cacheproxy

CMD ["/app/dist/cacheproxy"]
