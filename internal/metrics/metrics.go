// Package metrics centralizes Prometheus metrics used across collector, anchor,
// and API components. All metrics are registered in the default registry so
// the /metrics endpoint in the api binary exposes them automatically.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CollectorRunsTotal counts collector runs per source and status.
	CollectorRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "collector",
		Name:      "runs_total",
		Help:      "Total collector runs by source and outcome.",
	}, []string{"source", "status"})

	// CollectorRecordsTotal counts records persisted per source.
	CollectorRecordsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "collector",
		Name:      "records_total",
		Help:      "Total records persisted by collector.",
	}, []string{"source"})

	// CollectorRunDuration observes collector run wall-clock duration.
	CollectorRunDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "fepublica",
		Subsystem: "collector",
		Name:      "run_duration_seconds",
		Help:      "Wall-clock duration of collector runs by source.",
		Buckets:   prometheus.ExponentialBuckets(5, 2, 10), // 5s .. ~2.5h
	}, []string{"source"})

	// AnchorBuildRootsTotal counts Merkle roots computed.
	AnchorBuildRootsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "anchor",
		Name:      "merkle_roots_total",
		Help:      "Total Merkle roots computed.",
	})

	// AnchorSubmitTotal counts anchor submissions per calendar and outcome.
	AnchorSubmitTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "anchor",
		Name:      "submit_total",
		Help:      "Total anchor submissions to OTS calendars.",
	}, []string{"calendar", "status"})

	// AnchorUpgradeTotal counts upgrade attempts per calendar and outcome.
	AnchorUpgradeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "anchor",
		Name:      "upgrade_total",
		Help:      "Total anchor upgrade attempts to OTS calendars.",
	}, []string{"calendar", "status"})

	// SnapshotsWithoutRoot is a gauge of snapshots missing Merkle roots.
	SnapshotsWithoutRoot = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "fepublica",
		Subsystem: "anchor",
		Name:      "snapshots_without_root",
		Help:      "Number of snapshots still waiting for Merkle root computation.",
	})

	// AnchorsPendingUpgrade is a gauge of anchors waiting for Bitcoin confirmation.
	AnchorsPendingUpgrade = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "fepublica",
		Subsystem: "anchor",
		Name:      "pending_upgrade",
		Help:      "Number of anchors pending OpenTimestamps upgrade (Bitcoin commit).",
	})

	// APIRequestsTotal counts HTTP requests served by the API.
	APIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fepublica",
		Subsystem: "api",
		Name:      "requests_total",
		Help:      "Total HTTP requests handled by the API.",
	}, []string{"route", "status"})

	// APIRequestDuration observes HTTP request duration per route.
	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "fepublica",
		Subsystem: "api",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration per route.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"route"})
)
