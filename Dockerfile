FROM registry.suse.com/bci/golang:1.19 as go-builder
WORKDIR /app/
COPY . /app/

RUN zypper -n ref && \
    zypper -n in --allow-downgrade libgpgme-devel libassuan-devel libbtrfs-devel device-mapper-devel awk
RUN go build ./bin/analyzer && go build ./bin/storage
RUN set -eux; \
    # these are the rpm dependencies of the analyzer, in the next lines we check this on x86_64 only, because ldd fails in qemu…
    to_download=(libgpgme11 libassuan0 libgpg-error0 libdevmapper1_03 libselinux1 libudev1 libpcre1); \
    if [[ "$(uname -m)" = "x86_64" ]]; then \
        deps=(); \
        for lib in $(ldd analyzer |grep '=>'|awk '{print $3}'); do \
            pkg=$(rpm -q --qf "%{NAME}\n" --whatprovides $lib); \
            if [[ ! $pkg =~ glibc ]]; then deps+=( "$pkg" ); fi; \
        done; \
        [[ $(echo ${to_download[@]} ${deps[@]}|tr ' ' '\n' | sort | uniq -u) = "" ]]; \
    fi; \
    for pkg in "${to_download[@]}"; do zypper -n download $pkg; done

FROM registry.suse.com/bci/node:16 as node-builder
WORKDIR /app/
COPY . /app/

RUN npm -g install yarn && yarn install && yarn run buildProduction


FROM registry.suse.com/bci/bci-micro:15.4 as storage-backend-deploy
WORKDIR /app/
COPY --from=go-builder /app/storage .

EXPOSE 4040

ENTRYPOINT ["/app/storage"]


FROM registry.suse.com/bci/bci-minimal:15.4 as deploy
WORKDIR /app/
COPY --from=go-builder /app/analyzer .
COPY --from=node-builder /app/public/ public/
COPY --from=go-builder /var/cache/zypp/packages/SLE_BCI/ .

RUN rpm -i --nodeps --force $(uname -m)/*rpm && rm -rf $(uname -m)/ noarch
RUN mkdir -p /etc/containers/ /var/lib/containers/storage /var/run/containers/storage && \
    echo '{"default":[{"type":"insecureAcceptAnything"}],"transports":{"docker-daemon":{"":[{"type":"insecureAcceptAnything"}]}}}' > /etc/containers/policy.json && \
    echo $'[storage] \n\
driver = "vfs" \n\
runroot = "/var/run/containers/storage" \n\
graphroot = "/var/lib/containers/storage"' > /etc/containers/storage.conf

EXPOSE 5050

ENTRYPOINT ["/app/analyzer", "--no-rootless"]
