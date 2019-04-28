FROM golang:1.9

WORKDIR /go/src/app

COPY main.go main.go

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["app"]