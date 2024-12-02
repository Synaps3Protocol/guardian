FROM golang:1.23.3

WORKDIR /
# See details: https://github.com/ipfs/go-ds-s3
ENV GO111MODULE on

COPY go.mod ./
COPY go.sum ./
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Mirror path
COPY . .
RUN make build output=/main input=/cmd/server/main.go
CMD ["/main"]




