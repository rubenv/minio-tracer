FROM golang:alpine AS builder
WORKDIR /go/src/github.com/rubenv/minio-tracer
ADD . .
RUN go build -v -x -o minio-tracer .

FROM minio/mc
COPY --from=builder /go/src/github.com/rubenv/minio-tracer/minio-tracer /usr/bin
ENTRYPOINT []
CMD ["/usr/bin/minio-tracer"]
