FROM golang:1.23.3 AS builder

WORKDIR /

# See details: https://github.com/ipfs/go-ds-s3
ENV GO111MODULE on
# force package direct download
ENV GOPROXY direct

# We clone Kubo source code.
RUN git clone https://github.com/ipfs/kubo --branch release-v0.32.1
ENV SRC_DIR /kubo

# Move to kubo folder
WORKDIR $SRC_DIR

# Install the plugin and build ipfs
# Build IPFS with s3ds bundled
RUN go get -u github.com/ipfs/go-ds-s3@go-ds-s3-plugin/v0.32.1
# 0 means at the first in the order
# https://github.com/ipfs/kubo/blob/master/plugin/loader/preload.sh
RUN echo "\ns3ds github.com/ipfs/go-ds-s3/plugin 0" >> plugin/loader/preload_list
RUN make build || : #first build will fail
RUN go mod tidy
RUN make build
RUN make install

# ------------------------------------------------------
# From here a new base is created as an isolated context
# The actual IPFS image we will use
FROM ipfs/kubo:v0.32.1
# the same directory in builder
ENV SRC_DIR /kubo

# We copy the new binaries we built in the 'builder' stage (--from=builder)
COPY --from=builder $SRC_DIR/cmd/ipfs/ipfs /usr/local/bin/ipfs
COPY --from=builder $SRC_DIR/bin/container_daemon /usr/local/bin/start_ipfs
COPY --from=builder $SRC_DIR/bin/container_init_run /usr/local/bin/container_init_run

# init.d script IPFS runs before starting the daemon. Used to manipulate the IPFS config file.
COPY init.private.sh /container-init.d/001-config.sh
# Fix permissions on start_ipfs
RUN chmod 0755 /usr/local/bin/start_ipfs
RUN chown ipfs:users /container-init.d/001-config.sh

