FROM golang:1.23.3

WORKDIR /
ENV GO111MODULE on
COPY . .

RUN make build output=/main input=/cmd/node/main.go
CMD ["/main"]




