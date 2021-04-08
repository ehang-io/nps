FROM golang:1.15 as builder
ARG GOPROXY=direct
WORKDIR /go/src/ehang.io/nps
COPY . .
RUN go get -d -v ./... 
RUN CGO_ENABLED=0 go build -ldflags="-w -s -extldflags -static" ./cmd/nps/nps.go

FROM scratch
COPY --from=builder /go/src/ehang.io/nps/nps /
COPY --from=builder /go/src/ehang.io/nps/web /web
VOLUME /conf
CMD ["/nps"]
