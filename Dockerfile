# Build Stage
FROM lacion/docker-alpine:gobuildimage AS build-stage

LABEL app="build-ccrawl"
LABEL REPO="https://github.com/tricky42/ccrawl"

ENV GOROOT=/usr/lib/go \
    GOPATH=/gopath \
    GOBIN=/gopath/bin \
    PROJPATH=/gopath/src/github.com/tricky42/ccrawl

# Because of https://github.com/docker/docker/issues/14914
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin

ADD . /gopath/src/github.com/tricky42/ccrawl
WORKDIR /gopath/src/github.com/tricky42/ccrawl

RUN make build-alpine

# Final Stage
FROM lacion/docker-alpine:latest

ARG GIT_COMMIT
ARG VERSION
LABEL REPO="https://github.com/tricky42/ccrawl"
LABEL GIT_COMMIT=$GIT_COMMIT
LABEL VERSION=$VERSION

# Because of https://github.com/docker/docker/issues/14914
ENV PATH=$PATH:/opt/ccrawl/bin

WORKDIR /opt/ccrawl/bin

COPY --from=build-stage /gopath/src/github.com/tricky42/ccrawl/bin/ccrawl /opt/ccrawl/bin/
RUN chmod +x /opt/ccrawl/bin/ccrawl

CMD /opt/ccrawl/bin/ccrawl