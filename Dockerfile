FROM alpine:latest

ENTRYPOINT ["/bin/docker-inspect"]
CMD [""]

EXPOSE 2204

ENV BIND_PORT=2204 \
  DOCKER_HOST=unix:///var/run/docker.sock \
  DOCKER_TLS_VERIFY=0 \
  DOCKER_TLS_CACERT="" \
  DOCKER_TLS_CERT="" \
  DOCKER_TLS_KEY=""

COPY ./build/docker-inspect /bin/docker-inspect
