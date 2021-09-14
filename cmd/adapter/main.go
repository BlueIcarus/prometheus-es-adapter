package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/BlueIcarus/prometheus-es-adapter/pkg/elasticsearch"
	"github.com/BlueIcarus/prometheus-es-adapter/pkg/handlers"
	"github.com/BlueIcarus/prometheus-es-adapter/pkg/logger"
	"github.com/TV4/graceful"
	"github.com/avast/retry-go/v3"
	gorilla "github.com/gorilla/handlers"
	"github.com/namsral/flag"
	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

var (
	// Build number populated during build
	Build string
	// Commit hash populated during build
	Commit string
	// Global Elastic client
	client     *elastic.Client
	cert       tls.Certificate
	caCertPool *x509.CertPool = x509.NewCertPool()

	err error
	rtc = 1
)

func main() {
	var (
		urls              = flag.String("es_url", "http://localhost:9200", "ElasticSearch URL")
		user              = flag.String("es_user", "", "ElasticSearch user")
		pass              = flag.String("es_password", "", "ElasticSearch user password.")
		workers           = flag.Int("es_workers", 1, "Number of batch workers.")
		batchMaxAge       = flag.Int("es_batch_max_age", 10, "Max period in seconds between bulk ElasticSearch insert operations")
		batchMaxDocs      = flag.Int("es_batch_max_docs", 1000, "Max items for bulk ElasticSearch insert operation")
		batchMaxSize      = flag.Int("es_batch_max_size", 4096, "Max size in bytes for bulk ElasticSearch insert operation")
		indexAlias        = flag.String("es_alias", "prom-metrics", "ElasticSearch alias pointing to active write index")
		indexDaily        = flag.Bool("es_index_daily", false, "Create daily indexes and disable index management service")
		indexShards       = flag.Int("es_index_shards", 5, "Number of ElasticSearch shards to create per index")
		indexReplicas     = flag.Int("es_index_replicas", 1, "Number of ElasticSearch replicas to create per index")
		indexMaxAge       = flag.String("es_index_max_age", "7d", "Max age of ElasticSearch index before rollover")
		indexMaxDocs      = flag.Int64("es_index_max_docs", 1000000, "Max number of docs in ElasticSearch index before rollover")
		indexMaxSize      = flag.String("es_index_max_size", "", "Max size of index before rollover eg 5gb")
		loadIndexTemplate = flag.Bool("es_index_template", true, "Load the ElasticSearch index Template")
		searchMaxDocs     = flag.Int("es_search_max_docs", 1000, "Max number of docs returned for ElasticSearch search operation")
		sniffEnabled      = flag.Bool("es_sniff", false, "Enable ElasticSearch sniffing")
		tlsCertFile       = flag.String("es_tls_cert", "", "Location of the TLS cert file (es_tls_key required)")
		tlsKeyFile        = flag.String("es_tls_key", "", "Location of the TLS key file (es_tls_cert required)")
		tlsCaFile         = flag.String("es_tls_ca", "", "Location of the TLS ca file")
		retryCount        = flag.Uint("es_retry_count", 10, "Amount of retries to connect to ElasticSearch before exiting")
		retryDelay        = flag.Duration("es_retry_delay", 10, "Amount of seconds to wait between retries to connect to ElasticSearch before exiting")
		prometheusPort    = flag.Int("prometheus_port", 8000, "Port that Prometheus connects to for read/write")
		adminPort         = flag.Int("admin_port", 9000, "Port that Prometheus connects to for read/write")
		statsEnabled      = flag.Bool("stats", true, "Expose Prometheus metrics endpoint")
		debug             = flag.Bool("debug", false, "Debug logging")
	)
	flag.Parse()

	log := logger.NewLogger(*debug)

	log.Info(fmt.Sprintf("Starting commit: %+v, build: %+v", Commit, Build))
	log.Info(fmt.Sprintf("ElasticSearch max retry attempts: %d", *retryCount))
	log.Info(fmt.Sprintf("ElasticSearch retry delay: %d", *retryDelay))

	if *urls == "" {
		log.Fatal("Missing ElasticSearch URL")
	}

	ctx := context.TODO()

	u, err := url.Parse(*urls)
	if err != nil {
		log.Error("Could not determine scheme from url", zap.Error(err))
	}
	scheme := u.Scheme
	log.Info(fmt.Sprintf("Setting ElasticSearch scheme to: %+v", scheme))

	if *tlsCertFile != "" || *tlsKeyFile != "" {
		tlsCF, err := os.OpenFile(*tlsCertFile, os.O_RDONLY, 0666)
		if err != nil {
			if os.IsPermission(err) {
				log.Error("Unable to read TLS certificate file", zap.Error(err))
			}
		}
		tlsCF.Close()
		tlsKF, err := os.OpenFile(*tlsKeyFile, os.O_RDONLY, 0666)
		if err != nil {
			if os.IsPermission(err) {
				log.Error("Unable to read TLS key file", zap.Error(err))
			}
		}
		tlsKF.Close()
		cert, err = tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			log.Error("Could not process es_tls_cert/es_tls_key files", zap.Error(err))
		} else {
			log.Info("Loaded es_tls_cert/es_tls_key files")
		}
	}

	if *tlsCaFile != "" {
		caCert, err := ioutil.ReadFile(*tlsCaFile)
		if err != nil {
			log.Error("Could not process es_tls_ca file", zap.Error(err))
		} else {
			//caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			if caCertPool == nil {
				log.Error("Failed to populate CA pool")
			} else {
				log.Info("Loaded es_tls_ca file")
			}
		}
	}

	if cert.Certificate != nil && caCertPool != nil {
		log.Info("Using certificate authenticated https ElasticSearch cluster")
		log.Debug("All of es_tls_cert, es_tls_key and es_tls_ca are defined and setup correctly")
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}

		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: 5 * time.Second,
		}

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   time.Second * 10,
		}
		err := retry.Do(
			func() error {
				client, err = elastic.NewClient(elastic.SetHttpClient(httpClient),
					elastic.SetURL(*urls),
					elastic.SetBasicAuth(*user, *pass),
					elastic.SetScheme(scheme),
					elastic.SetSniff(*sniffEnabled),
				)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to create Elastic connection. Will retry: %d/%d", rtc, *retryCount), zap.Error(err))
					rtc++
					return err
				}
				defer client.Stop()

				return nil
			},
			retry.Attempts(*retryCount),
			retry.Delay(*retryDelay),
			retry.DelayType(retry.FixedDelay),
			retry.LastErrorOnly(true),
		)
		if err != nil {
			log.Fatal("Failed to create Elastic secure client", zap.Error(err))
		}
	} else if cert.Certificate != nil {
		log.Info("Using certificate authenticated ElasticSearch cluster")
		log.Debug("es_tls_cert and es_tls_key are defined and setup correctly")
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		}

		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: 5 * time.Second,
		}

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   time.Second * 10,
		}
		err := retry.Do(
			func() error {
				client, err = elastic.NewClient(elastic.SetHttpClient(httpClient),
					elastic.SetURL(*urls),
					elastic.SetBasicAuth(*user, *pass),
					elastic.SetScheme(scheme),
					elastic.SetSniff(*sniffEnabled),
				)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to create Elastic connection. Will retry: %d/%d", rtc, *retryCount), zap.Error(err))
					rtc++
					return err
				}
				defer client.Stop()

				return nil
			},
			retry.Attempts(*retryCount),
			retry.Delay(*retryDelay),
			retry.DelayType(retry.FixedDelay),
			retry.LastErrorOnly(true),
		)
		if err != nil {
			log.Fatal("Failed to create Elastic secure client", zap.Error(err))
		}
	} else if caCertPool != nil {
		log.Info("Using https ElasticSearch cluster with CA file")
		log.Debug("es_tls_ca is defined and setup correctly")
		tlsConfig := &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}

		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: 5 * time.Second,
		}

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   time.Second * 10,
		}

		err := retry.Do(
			func() error {
				client, err = elastic.NewClient(elastic.SetHttpClient(httpClient),
					elastic.SetURL(*urls),
					elastic.SetBasicAuth(*user, *pass),
					elastic.SetScheme(scheme),
					elastic.SetSniff(*sniffEnabled),
				)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to create Elastic connection. Will retry: %d/%d", rtc, *retryCount), zap.Error(err))
					rtc++
					return err
				}
				defer client.Stop()

				return nil
			},
			retry.Attempts(*retryCount),
			retry.Delay(*retryDelay),
			retry.DelayType(retry.FixedDelay),
			retry.LastErrorOnly(true),
		)

		if err != nil {
			log.Fatal("Failed to create Elastic secure client", zap.Error(err))
		}
	} else {
		log.Info("Using non-secure ElasticSearch cluster")
		err := retry.Do(
			func() error {
				client, err = elastic.NewClient(
					elastic.SetURL(*urls),
					elastic.SetBasicAuth(*user, *pass),
					elastic.SetScheme(scheme),
					elastic.SetSniff(*sniffEnabled),
				)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to create Elastic connection. Will retry: %d/%d", rtc, *retryCount), zap.Error(err))
					rtc++
					return err
				}
				defer client.Stop()

				return nil
			},
			retry.Attempts(*retryCount),
			retry.Delay(*retryDelay),
			retry.DelayType(retry.FixedDelay),
			retry.LastErrorOnly(true),
		)

		if err != nil {
			log.Fatal("Failed to create Elastic client", zap.Error(err))
		}
	}

	if client == nil {
		log.Fatal("Elastic client connection has invalid state", zap.Error(err))
	}

	if *loadIndexTemplate {
		err = elasticsearch.EnsureIndexTemplate(ctx, client, &elasticsearch.IndexTemplateConfig{
			Alias:    *indexAlias,
			Shards:   *indexShards,
			Replicas: *indexReplicas,
		})
		if err != nil {
			log.Fatal("Failed to create index template", zap.Error(err))
		}
	}

	if !*indexDaily {
		_, err = elasticsearch.NewIndexService(ctx, log, client, &elasticsearch.IndexConfig{
			Alias:   *indexAlias,
			MaxAge:  *indexMaxAge,
			MaxDocs: *indexMaxDocs,
			MaxSize: *indexMaxSize,
		})
		if err != nil {
			log.Fatal("Failed to create indexer", zap.Error(err))
		}
	}

	readCfg := &elasticsearch.ReadConfig{
		Alias:   *indexAlias,
		MaxDocs: *searchMaxDocs,
	}
	readSvc := elasticsearch.NewReadService(log, client, readCfg)

	writeCfg := &elasticsearch.WriteConfig{
		Alias:   *indexAlias,
		Daily:   *indexDaily,
		MaxAge:  *batchMaxAge,
		MaxDocs: *batchMaxDocs,
		MaxSize: *batchMaxSize,
		Workers: *workers,
		Stats:   *statsEnabled,
	}
	writeSvc, err := elasticsearch.NewWriteService(ctx, log, client, writeCfg)
	if err != nil {
		log.Fatal("Unable to create elasticsearch adapter:", zap.Error(err))
	}
	defer writeSvc.Close()

	// Create an "admin" listener on 0.0.0.0:adminPort
	go http.ListenAndServe(":"+fmt.Sprintf("%+v", *adminPort), handlers.NewAdminRouter(client))
	log.Info(fmt.Sprintf("Starting admin listener on port: %d", *adminPort))

	graceful.ListenAndServe(&http.Server{
		Addr: ":" + fmt.Sprintf("%+v", *prometheusPort),
		Handler: gorilla.RecoveryHandler(gorilla.PrintRecoveryStack(true))(
			gorilla.CompressHandler(
				handlers.NewRouter(writeSvc, readSvc),
			),
		),
	})
	log.Info(fmt.Sprintf("Starting prometheus listener on port: %d", *prometheusPort))
	// TODO: graceful shutdown of bulk processor
}
