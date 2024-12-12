FROM golang:1.23.3

WORKDIR /
ENV GO111MODULE on
ENV CGO_ENABLED 0
ENV GOOS linux

COPY . .
RUN go build -v -o /main /cmd/node/main.go

EXPOSE 5926/tcp

CMD ["/main"]




