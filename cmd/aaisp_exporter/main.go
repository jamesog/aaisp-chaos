package main

import (
	"flag"
	"fmt"
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

func usage(fs *flag.FlagSet) func() {
	return func() {
		o := fs.Output()
		fmt.Fprintf(o, "Usage:\n    %s ", os.Args[0])
		fs.VisitAll(func(f *flag.Flag) {
			s := fmt.Sprintf(" [-%s", f.Name)
			if arg, _ := flag.UnquoteUsage(f); len(arg) > 0 {
				s += " " + arg
			}
			s += "]"
			fmt.Fprint(o, s)
		})
		fmt.Fprint(o, "\n\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprint(o, "\nThe environment variables CHAOS_CONTROL_LOGIN and CHAOS_CONTROL_PASSWORD must be set.\n")
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
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.Usage = usage(fs)
	var (
		listen    = fs.String("listen", ":8080", "listen `address`")
		logLevel  = fs.String("log.level", "info", "log `level`")
		logOutput = fs.String("log.output", "json", "log output `style` (json, console)")
	)
	fs.Parse(os.Args[1:])

	log := setupLogger(*logLevel, *logOutput)

	var (
		controlLogin    = os.Getenv("CHAOS_CONTROL_LOGIN")
		controlPassword = os.Getenv("CHAOS_CONTROL_PASSWORD")
	)
	switch {
	case controlLogin == "" && controlPassword == "":
		log.Fatal().Msg("CHAOS_CONTROL_LOGIN and CHAOS_CONTROL_PASSWORD must be set in the environment")
	case controlLogin == "":
		log.Fatal().Msg("CHAOS_CONTROL_LOGIN is not set")
	case controlPassword == "":
		log.Fatal().Msg("CHAOS_CONTROL_PASSWORD is not set")
	}

	collector := broadbandCollector{
		API: chaos.New(chaos.Auth{
			ControlLogin:    controlLogin,
			ControlPassword: controlPassword,
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
