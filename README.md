# prometheus-es-adapter

*Please note:* This project is a copy of [https://github.com/pwillie/prometheus-es-adapter](https://github.com/pwillie/prometheus-es-adapter).
Credits are documented in the [AUTHORS.md](AUTHORS.md) file (which is an updated version of the original version from pwillie)

## Overview

A read and write adapter for prometheus persistent storage.

#### Exposed Endpoints

NOTE: The ports mentioned here are the default ports. Please modify your request accordingly.

| Port | Path     | Description                                                                             |
| ---- | -------- | --------------------------------------------------------------------------------------- |
| 8000 | /read    | Prometheus remote read endpoint                                                         |
| 8000 | /write   | Prometheus remote write endpoint                                                        |
| 9000 | /metrics | Surface Prometheus metrics                                                              |
| 9000 | /live    | Http probe endpoint to reflect service liveness                                         |
| 9000 | /ready   | Http probe endpoint reflecting the connection to and state of the ElasticSearch cluster |

## Config

The configuration parameters mentioned below can both be added as environment variables or arguments.

For example:
```Environment variable:
ES_URL=http://elasticsearch:9200 ./prometheus-es-adapter
```
```Command line argument:
./prometheus-es-adapter --es_url=http://elasticsearch:9200
```

| Env Variables      | Default               | Description                                                                      |
| -----------------  | --------------------- | -------------------------------------------------------------------------------- |
| ES_URL             | http://localhost:9200 | ElasticSearch URL                                                                |
| ES_USER            |                       | ElasticSearch User                                                               |
| ES_PASSWORD        |                       | ElasticSearch User Password                                                      |
| ES_WORKERS         | 1                     | Number of batch workers                                                          |
| ES_BATCH_MAX_AGE   | 10                    | Max period in seconds between bulk ElasticSearch insert operations               |
| ES_BATCH_MAX_DOCS  | 1000                  | Max items for bulk ElasticSearch insert operation                                |
| ES_BATCH_MAX_SIZE  | 4096                  | Max size in bytes for bulk ElasticSearch insert operation                        |
| ES_ALIAS           | prom-metrics          | ElasticSearch alias pointing to active write index                               |
| ES_INDEX_DAILY     | false                 | Create daily indexes and disable index rollover                                  |
| ES_INDEX_SHARDS    | 5                     | Number of ElasticSearch shards to create per index                               |
| ES_INDEX_REPLICAS  | 1                     | Number of ElasticSearch replicas to create per index                             |
| ES_INDEX_MAX_AGE   | 7d                    | Max age of ElasticSearch index before rollover                                   |
| ES_INDEX_MAX_DOCS  | 1000000               | Max number of docs in ElasticSearch index before rollover                        |
| ES_INDEX_MAX_SIZE  |                       | Max size of index before rollover eg 5gb                                         |
| ES_SEARCH_MAX_DOCS | 1000                  | Max number of docs returned for ElasticSearch search operation                   |
| ES_SNIFF           | false                 | Enable ElasticSearch sniffing                                                    |
| ES_TLS_CERT        |                       | Add a certificate file for ElasticSearch authentication (Requires: ES_TLS_KEY)   |
| ES_TLS_KEY         |                       | Add a key file for ElasticSearch authentication (Requires: ES_TLS_CERT)          |
| ES_TLS_CA          |                       | Add a CA file for ElasticSearch endpoint                                         |
| ES_RETRY_COUNT     | 10                    | Amount of retries for ElasticSearch connection                                   |
| ES_RETRY_DELAY     | 10                    | Amount of seconds between retries for ElasticSearch connection                   |
| PROMETHEUS_PORT    | 8000                  | Port that Prometheus connects to                                                 |
| ADMIN_PORT         | 9000                  | Port that admin context is mapped to                                             |
| STATS              | true                  | Expose Prometheus metrics endpoint                                               |
| DEBUG              | false                 | Display extra debug logs                                                         |

## Notes

Although *prometheus-es-adapter* will create and rollover ElasticSearch indexes it is expected that a tool such as ElasticSearch Curator will be used to maintain quiescent indexes eg deleting, shrinking and merging old indexes.

## TODO list

1. Add tests

## Changelog
### v1.0.2-7
1. Added option to disable ElasticSearch Index Template creation
2. Fixed an issue with the index template and did some proper indentation for better readability

### v1.0.1-7
1. Added some additional messages related to certificate loading
2. Fixed an issue with the ca root certificate loading

### v1.0.0-7
1. `Dockerfile`: Migrated Docker image from `Alpine` to `distroless`;
2. `Docker-compose.yml`:
   1. Updated containers to the latest version (and added versions where they were missing);
   2. Updated configuration to use secure ElasticSearch cluster.
3. `AUTHORS.md`: Updated content;
4. `pkg/elasticsearch/const.go`: Updated ElasticSearch index template to be compatible with ElasticSearch version 7;
5. `pkg/elasticsearch/index.go`:
   1. Updated ElasticSearch dependency to version 7;
   2. Migrated from Legacy ElasticSearch Template to composable ElasticSearch Template.
6. `pkg/elasticsearch/read.go`:
   1. Updated ElasticSearch dependency to version 7;
   2. Fixed a bug after the ElasticSearch dependency update;
   3. Enabled debug log message.
7. `pkg/elasticsearch/write.go`: Updated ElasticSearch dependency to version 7;
8. `pkg/handlers/health.go`: Updated ElasticSearch dependency to version 7;
9. `pkg/handlers/router.go`: Updated ElasticSearch dependency to version 7;
10. `cmd/adapter/main.go`:
    1. Updated ElasticSearch dependency to version 7;
    2. Added and updated some log messages;
    3. Added parameter to specify CA certificate file;
    4. Added parameter to specify cert and key file;
    5. Added parameter for a retry amount function on the ElasticSearch connect;
    6. Added parameter for a retry delay function on the ElasticSearch connect;
    7. Added parameter to specify Prometheus connect port;
    8. Added parameter to specify Admin connect port;
11. `go.mod`: Updated some packages and migrated to GO 1.7;

## Requirements

* 7.x ElasticSearch cluster

## Getting started

Automated builds of Docker image are available at https://hub.docker.com/r/icarus87/prometheus-es-adapter/.

## Contributing

Local development requires Go to be installed. On OS X with Homebrew you can just run `brew install go`.

Running it then should be as simple as:

```console
$ make build
$ ./release/linux/amd64/prometheus-es-adapter
```

### Testing

`make test`

### Local development

To start a local development environment run the commands below to start a ElasticSearch cluster.
```
docker-compose up -d --build
docker-compose logs -f prometheus-es-adapter
```

A empty ElasticSearch cluster will be setup. Below details apply for the cluster:
- Username: elastic
- Password: elastic
- ElasticSearch URL: https://elasticsearch:9200
- Kibana URL: https://localhost:5601
