# AAISP Exporter

A Prometheus exporter for information about [Andrews and Arnold](https://aa.net.uk) broadband lines.

It exposes metrics:

* **aaisp_broadband_quota_remaining**: The line's remaining in the current monthly quota in bytes
* **aaisp_broadband_quota_total**: The line's monthly quota in bytes, excluding rollover
* **aaisp_broadband_rx_rate**: The line's receive (upload) rate in bits per second
* **aaisp_broadband_tx_rate**: The line's transmit (download) rate in bits per second

To run the service you must export environment variables `CHAOS_CONTROL_LOGIN` and `CHAOS_CONTROL_PASSWORD` using the login details you use for https://control.aa.net.uk/.

The service takess a `-listen` flag for setting the address and port the service binds to. The default is `:8080`.
