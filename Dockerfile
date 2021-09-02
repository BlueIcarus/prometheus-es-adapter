# Build Stage
FROM golang:1.17 AS build-stage

WORKDIR /src
ADD . .

RUN make test && make build

# Final Stage
FROM gcr.io/distroless/base-debian10

ENV USERID 789

ARG GIT_COMMIT
ARG VERSION
LABEL REPO="https://github.com/BlueIcarus/prometheus-es-adapter"
LABEL GIT_COMMIT=$GIT_COMMIT
LABEL VERSION=$VERSION

COPY --from=build-stage /src/release/linux/amd64/prometheus-es-adapter /usr/local/bin/

USER ${USERID}

CMD [ "prometheus-es-adapter" ]
