FROM registry.suse.com/bci/golang:1.17 as go-builder
WORKDIR /app/
COPY . /app/

RUN zypper -n ref && \
    zypper -n in --allow-downgrade libgpgme-devel libassuan-devel libbtrfs-devel device-mapper-devel awk
RUN go build
RUN for lib in $(ldd container-layer-sizes |grep '=>'|awk '{print $3}'); do \
        pkg=$(rpm -q --whatprovides $lib); \
        if [[ ! $pkg =~ glibc ]]; then zypper download $pkg; fi; \
    done

FROM registry.suse.com/bci/nodejs:14 as node-builder
WORKDIR /app/
COPY . /app/

RUN npm -g install yarn && yarn install && yarn run buildProduction

FROM registry.suse.com/bci/minimal:15.3 as deploy
WORKDIR /app/
COPY --from=go-builder /app/container-layer-sizes .
COPY --from=node-builder /app/dist/ dist/
COPY --from=go-builder /var/cache/zypp/packages/SLE_BCI/x86_64/ .

RUN rpm -i --nodeps --force *rpm && rm -rf *rpm
RUN mkdir -p /etc/containers/ /var/lib/containers/storage /var/run/containers/storage && \
    echo '{"default":[{"type":"insecureAcceptAnything"}],"transports":{"docker-daemon":{"":[{"type":"insecureAcceptAnything"}]}}}' > /etc/containers/policy.json && \
    echo $'[storage] \n\
driver = "vfs" \n\
runroot = "/var/run/containers/storage" \n\
graphroot = "/var/lib/containers/storage"' > /etc/containers/storage.conf

EXPOSE 5050

ENTRYPOINT ["/app/container-layer-sizes", "--no-rootless"]
