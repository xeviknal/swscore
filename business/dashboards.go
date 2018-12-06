package business

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
)

// DashboardsService deals with fetching dashboards from k8s client
type DashboardsService struct {
	prom prometheus.ClientInterface
	mon  kubernetes.KialiMonitoringInterface
}

// NewDashboardsService initializes this business service
func NewDashboardsService(mon kubernetes.KialiMonitoringInterface, prom prometheus.ClientInterface) DashboardsService {
	return DashboardsService{prom: prom, mon: mon}
}

// GetDashboard returns a dashboard filled-in with target data
func (in *DashboardsService) GetDashboard(params prometheus.CustomMetricsQuery, template string) (*models.MonitoringDashboard, error) {
	dashboard, err := in.mon.GetDashboard(params.Namespace, template)
	if err != nil {
		// Dashboard might be in Kiali namespace
		cfg := config.Get()
		dashboard, err = in.mon.GetDashboard(cfg.IstioNamespace, template)
		if err != nil {
			return nil, err
		}
	}

	labels := fmt.Sprintf(`{namespace="%s",app="%s"`, params.Namespace, params.App)
	if params.Version != "" {
		labels += fmt.Sprintf(`,version="%s"`, params.Version)
	}
	labels += "}"
	grouping := strings.Join(params.ByLabels, ",")

	wg := sync.WaitGroup{}
	wg.Add(len(dashboard.Spec.Charts))
	filledCharts := make([]models.Chart, len(dashboard.Spec.Charts))

	for i, c := range dashboard.Spec.Charts {
		go func(idx int, chart kubernetes.MonitoringDashboardChart) {
			defer wg.Done()
			filledCharts[idx] = models.ConvertChart(chart)
			if chart.MetricType == "counter" {
				filledCharts[idx].CounterRate = in.prom.FetchRateRange(chart.MetricName, labels, grouping, &params.BaseMetricsQuery)
			} else {
				filledCharts[idx].Histogram = in.prom.FetchHistogramRange(chart.MetricName, labels, grouping, &params.BaseMetricsQuery)
			}
		}(i, c)
	}

	wg.Wait()
	return &models.MonitoringDashboard{
		Title:        dashboard.Spec.Title,
		Charts:       filledCharts,
		Aggregations: models.ConvertAggregations(dashboard.Spec),
	}, nil
}

type istioChart struct {
	models.Chart
	refName string
}

var istioCharts = []istioChart{
	istioChart{
		Chart: models.Chart{
			Name:  "Request volume",
			Unit:  "ops",
			Spans: 12,
		},
		refName: "request_count",
	},
	istioChart{
		Chart: models.Chart{
			Name:  "Request duration",
			Unit:  "s",
			Spans: 12,
		},
		refName: "request_duration",
	},
	istioChart{
		Chart: models.Chart{
			Name:  "Request size",
			Unit:  "B",
			Spans: 12,
		},
		refName: "request_size",
	},
	istioChart{
		Chart: models.Chart{
			Name:  "Response size",
			Unit:  "B",
			Spans: 12,
		},
		refName: "response_size",
	},
	istioChart{
		Chart: models.Chart{
			Name:  "TCP received",
			Unit:  "bps",
			Spans: 12,
		},
		refName: "tcp_received",
	},
	istioChart{
		Chart: models.Chart{
			Name:  "TCP sent",
			Unit:  "bps",
			Spans: 12,
		},
		refName: "tcp_sent",
	},
}

// GetIstioDashboard returns Istio dashboard (currently hard-coded) filled-in with metrics
func (in *DashboardsService) GetIstioDashboard(params prometheus.IstioMetricsQuery) (*models.MonitoringDashboard, error) {
	var dashboard models.MonitoringDashboard
	// Copy dashboard
	if params.Direction == "inbound" {
		dashboard = models.PrepareIstioDashboard("Inbound", "destination", "source")
	} else {
		dashboard = models.PrepareIstioDashboard("Outbound", "source", "destination")
	}

	metrics := in.prom.GetMetrics(&params)

	for _, chartTpl := range istioCharts {
		newChart := chartTpl.Chart
		if metric, ok := metrics.Metrics[chartTpl.refName]; ok {
			newChart.CounterRate = metric
		}
		if histo, ok := metrics.Histograms[chartTpl.refName]; ok {
			newChart.Histogram = histo
		}
		dashboard.Charts = append(dashboard.Charts, newChart)
	}

	return &dashboard, nil
}
