FROM golang:1.9.2

RUN curl -Lo /usr/local/bin/dep https://github.com/golang/dep/releases/download/v0.3.2/dep-linux-amd64 && \
  echo "322152b8b50b26e5e3a7f6ebaeb75d9c11a747e64bbfd0d8bb1f4d89a031c2b5  /usr/local/bin/dep" | sha256sum -c && \
  chmod +x /usr/local/bin/dep

WORKDIR /go/src/github.com/simonswine/gke-kubeconfig-builder

COPY Gopkg.lock .
COPY Gopkg.toml .
RUN dep ensure -vendor-only

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gke-kubeconfig-builder .

FROM alpine:3.7

RUN apk --no-cache add ca-certificates curl

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.9.0/bin/linux/amd64/kubectl && \
     chmod +x kubectl && \
     mv kubectl /usr/local/bin 

COPY --from=0 /go/src/github.com/simonswine/gke-kubeconfig-builder/gke-kubeconfig-builder /usr/local/bin
ENTRYPOINT ["/usr/local/bin/gke-kubeconfig-builder"]
