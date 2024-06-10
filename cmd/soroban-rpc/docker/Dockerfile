FROM golang:1.22-bullseye as build
ARG RUST_TOOLCHAIN_VERSION=stable
ARG REPOSITORY_VERSION

WORKDIR /go/src/github.com/HashCash-Consultants/soroban-rpc

ADD . ./

RUN git config --global --add safe.directory "/go/src/github.com/HashCash-Consultants/soroban-rpc"

ENV CARGO_HOME=/rust/.cargo
ENV RUSTUP_HOME=/rust/.rust
ENV PATH="/usr/local/go/bin:$CARGO_HOME/bin:${PATH}"
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update
RUN apt-get install -y build-essential
RUN apt-get clean

RUN curl https://sh.rustup.rs -sSf | sh -s -- -y --default-toolchain $RUST_TOOLCHAIN_VERSION

RUN make REPOSITORY_VERSION=${REPOSITORY_VERSION} build-soroban-rpc
RUN mv soroban-rpc /bin/soroban-rpc

FROM ubuntu:22.04
ARG HCNET_CORE_VERSION
ENV HCNET_CORE_VERSION=${HCNET_CORE_VERSION:-*}
ENV HCNET_CORE_BINARY_PATH /usr/bin/hcnet-core
ENV DEBIAN_FRONTEND=noninteractive

# ca-certificates are required to make tls connections
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl jq wget gnupg apt-utils
RUN wget -qO - https://apt.hcnet.org/SDF.asc | APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=true apt-key add -
RUN echo "deb https://apt.hcnet.org focal stable" >/etc/apt/sources.list.d/SDF.list
RUN echo "deb https://apt.hcnet.org focal unstable" >/etc/apt/sources.list.d/SDF-unstable.list
RUN apt-get update && apt-get install -y hcnet-core=${HCNET_CORE_VERSION}
RUN apt-get clean

COPY --from=build /bin/soroban-rpc /app/
ENTRYPOINT ["/app/soroban-rpc"]
