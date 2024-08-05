FROM docker.io/library/golang:bookworm AS build

COPY . /go/src/github.com/mrhaoxx/SOJ

RUN apt-get update && apt-get install -y build-essential

RUN cd /go/src/github.com/mrhaoxx/SOJ && go build -o /soj

FROM debian:bookworm AS runtime


COPY --from=build /soj /soj

ENTRYPOINT ["/soj"]
