FROM docker.io/library/golang:alpine AS build

COPY . /go/src/github.com/mrhaoxx/SOJ/subsystems/samplejudge

RUN cd /go/src/github.com/mrhaoxx/SOJ/subsystems/samplejudge && go build -o /samplejudge main.go

FROM docker.io/library/debian AS runtime

COPY --from=build /samplejudge /samplejudge

ENTRYPOINT ["/bin/sh", "-c", "sleep ${timeout:-1800}"]