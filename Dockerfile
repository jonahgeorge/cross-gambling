FROM golang:1.14
ENV PORT 80

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["cg"]
