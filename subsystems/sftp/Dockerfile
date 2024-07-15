FROM golang:alpine AS build

COPY . /go/src/github.com/mrhaoxx/SOJ/subsystems/sftp

RUN cd /go/src/github.com/mrhaoxx/SOJ/subsystems/sftp && go build -o /soj-sftp

FROM scratch AS runtime


COPY --from=build /soj-sftp /soj-sftp

EXPOSE 2207

ENTRYPOINT ["/soj-sftp"]
