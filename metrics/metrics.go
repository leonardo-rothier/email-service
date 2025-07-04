package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EmailsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "email_service_emails_processed_total",
		Help: "Total number of emails processed",
	}, []string{"sender", "code"})
)
