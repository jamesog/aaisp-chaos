package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"strconv"

	chaos "github.com/jamesog/aaisp-chaos"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

var (
	broadbandQuotaRemainingDesc = prometheus.NewDesc(
		"aaisp_broadband_quota_remaining",
		"Quota remaining in bytes",
		[]string{"line_id"},
		nil,
	)
	broadbandQuotaTotalDesc = prometheus.NewDesc(
		"aaisp_broadband_quota_total",
		"Quota total in bytes",
		[]string{"line_id"},
		nil,
	)
	broadbandTXRateDesc = prometheus.NewDesc(
		"aaisp_broadband_tx_rate",
		"Line transmit rate in bits per second",
		[]string{"line_id"},
		nil,
	)
	broadbandRXRateDesc = prometheus.NewDesc(
		"aaisp_broadband_rx_rate",
		"Line receive rate in bits per second",
		[]string{"line_id"},
		nil,
	)
	scrapeSuccessGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aaisp_scrape_success",
		Help: "Displays whether or not the AAISP API scrape was a success",
	})
)

type broadbandCollector struct {
	*chaos.API
	log zerolog.Logger
}

func (bc broadbandCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(bc, ch)
}

func (bc broadbandCollector) Collect(ch chan<- prometheus.Metric) {
	lines, err := bc.BroadbandInfo()
	if err != nil {
		bc.log.Debug().Err(err).Msg("error getting broadband info")
		scrapeSuccessGauge.Set(0)
		return
	}
	scrapeSuccessGauge.Set(1)
	for _, line := range lines {
		ch <- prometheus.MustNewConstMetric(
			broadbandQuotaRemainingDesc,
			prometheus.GaugeValue,
			float64(line.QuotaRemaining),
			strconv.Itoa(line.ID),
		)
		ch <- prometheus.MustNewConstMetric(
			broadbandQuotaTotalDesc,
			prometheus.CounterValue,
			float64(line.QuotaMonthly),
			strconv.Itoa(line.ID),
		)
		ch <- prometheus.MustNewConstMetric(
			broadbandTXRateDesc,
			prometheus.GaugeValue,
			float64(line.TXRate),
			strconv.Itoa(line.ID),
		)
		ch <- prometheus.MustNewConstMetric(
			broadbandRXRateDesc,
			prometheus.GaugeValue,
			float64(line.RXRate),
			strconv.Itoa(line.ID),
		)
	}
}

func loggingMiddleware(log zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				remoteHost = r.RemoteAddr
			}
			log.Log().
				Str("proto", r.Proto).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", remoteHost).
				Str("user_agent", r.Header.Get("User-Agent")).
				Send()
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func setupLogger(level, output string) zerolog.Logger {
	ll, err := zerolog.ParseLevel(level)
	if err != nil {
		ll = zerolog.InfoLevel
	}

	log := zerolog.New(os.Stderr).Level(ll).With().Timestamp().Logger()

	switch output {
	case "console":
		log = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	return log
}

func main() {
	var (
		listen    = flag.String("listen", ":8080", "listen `address`")
		logLevel  = flag.String("log.level", "info", "log `level`")
		logOutput = flag.String("log.output", "json", "log output style (json, console)")
	)
	flag.Parse()

	log := setupLogger(*logLevel, *logOutput)

	collector := broadbandCollector{
		API: chaos.New(chaos.Auth{
			ControlLogin:    os.Getenv("CHAOS_CONTROL_LOGIN"),
			ControlPassword: os.Getenv("CHAOS_CONTROL_PASSWORD"),
		}),
		log: log,
	}

	loggedHandler := loggingMiddleware(log)

	prometheus.MustRegister(collector)
	prometheus.MustRegister(scrapeSuccessGauge)
	http.Handle("/metrics", loggedHandler(promhttp.Handler()))
	log.Info().Msgf("Listening on %s", *listen)
	log.Fatal().Err(http.ListenAndServe(*listen, nil)).Send()
}
