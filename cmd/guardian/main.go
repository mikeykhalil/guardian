package main

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/dollarshaveclub/guardian/pkg/guardian"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {

	logLevel := kingpin.Flag("log-level", "log level.").Short('l').Default("warn").OverrideDefaultFromEnvar("LOG_LEVEL").String()
	address := kingpin.Flag("address", "host:port.").Short('a').Default("0.0.0.0:3000").OverrideDefaultFromEnvar("ADDRESS").String()
	redisAddress := kingpin.Flag("redis-address", "host:port.").Short('r').OverrideDefaultFromEnvar("REDIS_ADDRESS").String()
	redisPoolSize := kingpin.Flag("redis-pool-size", "redis connection pool size").Short('p').Default("20").OverrideDefaultFromEnvar("REDIS_POOL_SIZE").Int()
	dogstatsdAddress := kingpin.Flag("dogstatsd-address", "host:port.").Short('d').OverrideDefaultFromEnvar("DOGSTATSD_ADDRESS").String()
	reportOnly := kingpin.Flag("report-only", "report only, do not block.").Default("false").Short('o').OverrideDefaultFromEnvar("REPORT_ONLY").Bool()
	reqLimit := kingpin.Flag("limit", "request limit per duration.").Short('q').Default("10").OverrideDefaultFromEnvar("LIMIT").Uint64()
	limitDuration := kingpin.Flag("limit-duration", "duration to apply limit. supports time.ParseDuration format.").Short('y').Default("1s").OverrideDefaultFromEnvar("LIMIT_DURATION").Duration()
	limitEnabled := kingpin.Flag("limit-enabled", "rate limit enabled").Short('e').Default("true").OverrideDefaultFromEnvar("LIMIT_ENBALED").Bool()
	ingressClass := kingpin.Flag("ingress-class", "rate limit enabled").Short('c').Default("default").OverrideDefaultFromEnvar("INGRESS_CLASS").String()
	kingpin.Parse()

	logger := logrus.StandardLogger()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		level = logrus.ErrorLevel
	}

	logger.Warnf("setting log level to %v", level)
	logger.SetLevel(level)

	l, err := net.Listen("tcp", *address)
	if err != nil {
		logger.WithError(err).Errorf("could not listen on %s", *address)
		os.Exit(1)
	}

	var reporter guardian.MetricReporter
	if len(*dogstatsdAddress) == 0 {
		reporter = guardian.NullReporter{}
	} else {
		ddStatsd, err := statsd.NewBuffered(*dogstatsdAddress, 100)

		if err != nil {
			logger.WithError(err).Errorf("could create dogstatsd client with address %s", *dogstatsdAddress)
			os.Exit(1)
		}

		ddStatsd.Namespace = "guardian."
		reporter = &guardian.DataDogReporter{Client: ddStatsd, IngressClass: *ingressClass}
	}

	wg := sync.WaitGroup{}
	redisOpts := &redis.Options{
		Addr:         *redisAddress,
		PoolSize:     *redisPoolSize,
		DialTimeout:  guardian.DefaultRedisDialTimeout,
		ReadTimeout:  guardian.DefaultRedisReadTimeout,
		WriteTimeout: guardian.DefaultRedisWriteTimeout,
	}
	redis := redis.NewClient(redisOpts)

	redisWhitelistStore := guardian.NewRedisIPWhitelistStore(redis, logger)

	logger.Infof("starting cache update for whitelist store")
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		redisWhitelistStore.RunCacheUpdate(30*time.Second, stop)
	}()

	logger.Infof("setting whitelister to use redis store at %v", *redisAddress)
	whitelister := guardian.NewIPWhitelister(redisWhitelistStore, logger)

	limit := guardian.Limit{Count: *reqLimit, Duration: *limitDuration, Enabled: *limitEnabled}
	redisLimitStore := guardian.NewRedisLimitStore(limit, redis, logger.WithField("context", "redis"))
	logger.Infof("setting ip rate limiter to use redis store at %v with %v", *redisAddress, limit)
	rateLimiter := guardian.NewIPRateLimiter(redisLimitStore, logger.WithField("context", "ip-rate-limiter"))

	condWhitelistFunc := guardian.CondStopOnWhitelistFunc(whitelister)
	condRatelimitFunc := guardian.CondStopOnBlock(rateLimiter.Limit)
	condFuncChain := guardian.CondChain(condWhitelistFunc, condRatelimitFunc)

	logger.Infof("starting server on %v", *address)
	server := guardian.NewServer(condFuncChain, *reportOnly, logger.WithField("context", "server"), reporter)
	err = server.Serve(l)
	if err != nil {
		logger.WithError(err).Error("error running server")
	}

	logger.Debug("stopping server")

	close(stop)
	wg.Wait()

	logger.Debug("goodbye")
	if err != nil {
		os.Exit(1)
	}
}
