FROM registry.suse.com/bci/golang:1.17 as go-builder
WORKDIR /app/
COPY . /app/

RUN zypper -n ref && \
    zypper -n in --allow-downgrade libgpgme-devel libassuan-devel libbtrfs-devel device-mapper-devel awk
RUN go build ./bin/analyzer && go build ./bin/storage
RUN for lib in $(ldd analyzer |grep '=>'|awk '{print $3}'); do \
        pkg=$(rpm -q --whatprovides $lib); \
        if [[ ! $pkg =~ glibc ]]; then zypper download $pkg; fi; \
    done

FROM registry.suse.com/bci/node:16 as node-builder
WORKDIR /app/
COPY . /app/

RUN npm -g install yarn && yarn install && yarn run buildProduction


FROM registry.suse.com/bci/bci-micro:15.3 as storage-backend-deploy
WORKDIR /app/
COPY --from=go-builder /app/storage .

EXPOSE 4040

ENTRYPOINT ["/app/storage"]


FROM registry.suse.com/bci/bci-minimal:15.3 as deploy
WORKDIR /app/
COPY --from=go-builder /app/analyzer .
COPY --from=node-builder /app/public/ public/
COPY --from=go-builder /var/cache/zypp/packages/SLE_BCI/x86_64/ .

RUN rpm -i --nodeps --force *rpm && rm -rf *rpm
RUN mkdir -p /etc/containers/ /var/lib/containers/storage /var/run/containers/storage && \
    echo '{"default":[{"type":"insecureAcceptAnything"}],"transports":{"docker-daemon":{"":[{"type":"insecureAcceptAnything"}]}}}' > /etc/containers/policy.json && \
    echo $'[storage] \n\
driver = "vfs" \n\
runroot = "/var/run/containers/storage" \n\
graphroot = "/var/lib/containers/storage"' > /etc/containers/storage.conf

EXPOSE 5050

ENTRYPOINT ["/app/analyzer", "--no-rootless"]
