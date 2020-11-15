FROM golang:1.15 as build

WORKDIR /go/src/github.com/webdevops/kube-bootstrap-token-manager

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/kube-bootstrap-token-manager
COPY ./go.sum /go/src/github.com/webdevops/kube-bootstrap-token-manager
COPY ./Makefile /go/src/github.com/webdevops/kube-bootstrap-token-manager
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/kube-bootstrap-token-manager
RUN make test
RUN make lint
RUN make build
RUN ./kube-bootstrap-token-manager --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/base
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/kube-bootstrap-token-manager/kube-bootstrap-token-manager /
USER 1000
ENTRYPOINT ["/kube-bootstrap-token-manager"]
