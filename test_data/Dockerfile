FROM docker.io/library/alpine as builder

RUN mkdir -p /data/subdir
RUN for i in $(seq 100); do dd if=/dev/zero of=/data/$i count=$i; done
RUN for i in $(seq 100); do dd if=/dev/zero of=/data/subdir/$i count=$((i * 2)); done

FROM scratch as res1
ENV VERSION=3.0
LABEL foobar="42"
COPY --from=builder /data/ .

FROM scratch as res2
COPY --from=builder /data/subdir .
