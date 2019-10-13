package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"

	chaos "github.com/jamesog/aaisp-chaos"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
)

type broadbandCollector struct {
	*chaos.API
}

func (bc broadbandCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(bc, ch)
}

func (bc broadbandCollector) Collect(ch chan<- prometheus.Metric) {
	lines, err := bc.BroadbandInfo()
	if err != nil {
		log.Printf("error getting broadband info: %v\n", err)
		return
	}
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

func main() {
	listen := flag.String("listen", ":8080", "listen `address`")
	flag.Parse()

	collector := broadbandCollector{
		API: chaos.New(chaos.Auth{
			ControlLogin:    os.Getenv("CHAOS_CONTROL_LOGIN"),
			ControlPassword: os.Getenv("CHAOS_CONTROL_PASSWORD"),
		}),
	}

	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listen, nil))
}
