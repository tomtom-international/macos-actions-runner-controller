/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package metrics_aggregator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	tarter "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/rest"
)

const (
	cacheSize     = 5
	scrapeTimeout = 15 * time.Second
)

// TartVM holds information about a Tart VM and runner metadata
type TartVM struct {
	Name       string
	RunnerName string
	RunnerID   string
	IP         string
}

// MetricsResult holds the result of scraping a single VM
type MetricsResult struct {
	Error          error
	VM             TartVM
	MetricFamilies []*dto.MetricFamily
}

type MetricsAggregator struct {
	tarterClient      *tarter.Client
	tartClient        *tart.Client
	cache             *Cache
	aggregatorMetrics *AggregatorOwnMetrics

	scrapePort    string
	scrapeTimeout time.Duration
}

type AggregatorOwnMetrics struct {
	exporterInfo        *prometheus.GaugeVec
	lastScrapeTimestamp prometheus.Gauge
	totalVMCount        prometheus.Gauge
	successfulScrapes   prometheus.Gauge
}

func NewMetricsAggregator(tarterUrl, tartPath, scrapePort string) (*MetricsAggregator, error) {
	ownMetrics := &AggregatorOwnMetrics{}
	ownMetrics.exporterInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "macos_tart_vm_aggregator_info",
			Help: "Information about the MacOS Tart VMs metrics aggregator",
		},
		[]string{"version"},
	)

	ownMetrics.exporterInfo.WithLabelValues(coreVersion.GetVersionInfo().Version).Set(1)

	ownMetrics.lastScrapeTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "macos_tart_vm_aggregator_last_scrape_timestamp_seconds",
			Help: "Last scrape timestamp",
		},
	)

	ownMetrics.totalVMCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "macos_tart_vm_aggregator_vm_count",
			Help: "Total number of tart VMs discovered",
		},
	)

	ownMetrics.successfulScrapes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "macos_tart_vm_aggregator_successful_scrapes",
			Help: "Number of successful VM scrapes in the last collection cycle",
		},
	)

	tarterClient, err := tarter.NewClient(tarterUrl)
	if err != nil {
		logger.Errorf("Error creating tarter client: %v", err)
		return nil, err
	}

	return &MetricsAggregator{
		tarterClient:      tarterClient,
		tartClient:        tart.NewDefaultClient(tartPath),
		cache:             NewCache(cacheSize),
		aggregatorMetrics: ownMetrics,
		scrapePort:        scrapePort,
		scrapeTimeout:     scrapeTimeout,
	}, nil
}

func (ma *MetricsAggregator) CollectRunnersMetrics() (string, error) {
	vms, err := ma.getRunningVMs()
	if err != nil {
		logger.Errorf("Error getting runners from Tarter: %v", err)
		return "", err
	}

	if len(vms) == 0 {
		return "# No running Tart VMs found\n", nil
	}

	results := make(chan MetricsResult, len(vms))

	for _, vm := range vms {
		go func(vm TartVM) {
			logger.Debugf("Collecting metrics for Tart VM: %s", vm.Name)

			metricFamilies, err := ma.scrapeTartVMMetrics(vm)
			results <- MetricsResult{
				VM:             vm,
				MetricFamilies: metricFamilies,
				Error:          err,
			}
		}(vm)
	}

	var allMetricsFamilies []*dto.MetricFamily
	successfulScrapes := 0

	for i := 0; i < len(vms); i++ {
		result := <-results
		if result.Error != nil {
			logger.Warnf("Error scraping metrics from VM %s: %v", result.VM.Name, result.Error)
			continue
		}
		for _, family := range result.MetricFamilies {
			labeledFamily := ma.addRunnerLabelsToMetricFamily(family, result.VM)
			allMetricsFamilies = append(allMetricsFamilies, labeledFamily)
		}
		successfulScrapes++
	}

	// Update aggregator metrics
	ma.aggregatorMetrics.totalVMCount.Set(float64(len(vms)))
	ma.aggregatorMetrics.successfulScrapes.Set(float64(successfulScrapes))
	ma.aggregatorMetrics.lastScrapeTimestamp.Set(float64(time.Now().Unix()))

	aggregatorMetrics, err := ma.getAggregatorMetrics()
	if err != nil {
		logger.Warnf("Failed to get aggregator metrics: %v", err)
	} else {
		allMetricsFamilies = append(aggregatorMetrics, allMetricsFamilies...)
	}

	// Merge metric families with the same name
	mergedMetricsFamilies := ma.mergeMetricFamilies(allMetricsFamilies)

	metrics, err := ma.formatMetricsAsPrometheusText(mergedMetricsFamilies)
	if err != nil {
		return "", fmt.Errorf("failed to format metrics: %w", err)
	}

	return metrics, nil
}

// getAggregatorMetrics returns the aggregator's own metrics as DTO objects
func (ma *MetricsAggregator) getAggregatorMetrics() ([]*dto.MetricFamily, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(ma.aggregatorMetrics.exporterInfo)
	registry.MustRegister(ma.aggregatorMetrics.lastScrapeTimestamp)
	registry.MustRegister(ma.aggregatorMetrics.totalVMCount)
	registry.MustRegister(ma.aggregatorMetrics.successfulScrapes)

	metricFamilies, err := registry.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather aggregator metrics: %w", err)
	}

	return metricFamilies, nil
}

// getRunningVMs retrieves the list of running Tart VMs and their runner's metadata from Tarter API
func (ma *MetricsAggregator) getRunningVMs() ([]TartVM, error) {
	var vms []TartVM
	ctx, cancel := context.WithTimeout(context.Background(), ma.scrapeTimeout)
	defer cancel()
	runners, err := ma.tarterClient.GetActiveRunners(ctx)
	if err != nil {
		return vms, err
	}

	for _, runner := range runners {
		ip, err := ma.getTartVMIPs(runner.TartVMName)
		if err != nil {
			logger.Debugf("Unable to get IP for VM %s: %v", runner.TartVMName, err)
			continue
		}
		vms = append(vms, TartVM{
			Name:       runner.TartVMName,
			RunnerName: runner.GhaRunnerName,
			RunnerID:   runner.ID,
			IP:         ip,
		})
	}

	return vms, nil
}

// getTartVMIPs retrieves the IP address of a Tart VM from the cache
// or using tart client if IP is not cached and caches it
func (ma *MetricsAggregator) getTartVMIPs(vmName string) (string, error) {
	if ip, exists := ma.cache.Get(vmName); exists {
		logger.Debugf("Cache hit for VM %s: %s", vmName, ip)
		return ip, nil
	}

	logger.Debugf("Cache miss for VM %s", vmName)
	ip, err := ma.tartClient.GetTartVMIP(vmName)
	if err != nil {
		return "", err
	}
	ma.cache.Set(vmName, ip)
	logger.Debugf("Cached IP for VM %s: %s", vmName, ip)
	return ip, nil
}

// scrapeTartVMMetrics fetches and parses metrics from a single VM
func (ma *MetricsAggregator) scrapeTartVMMetrics(vm TartVM) ([]*dto.MetricFamily, error) {
	url := fmt.Sprintf("http://%s:%s/metrics", vm.IP, ma.scrapePort)

	ctx, cancel := context.WithTimeout(context.Background(), ma.scrapeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for VM %s: %w", vm.Name, err)
	}

	req.Header.Add("Accept", "application/openmetrics-text; version=1.0.0, text/plain")

	client := &http.Client{
		Timeout: ma.scrapeTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape metrics from VM %s: %w", vm.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from VM %s: %d", vm.Name, resp.StatusCode)
	}

	var parser expfmt.TextParser

	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics from VM %s: %w", vm.Name, err)
	}

	families := make([]*dto.MetricFamily, 0, len(metricFamilies))
	for _, family := range metricFamilies {
		families = append(families, family)
	}

	return families, nil
}

// addRunnerLabelsToMetricFamily adds Runner specific labels to all metrics in a metric family
func (ma *MetricsAggregator) addRunnerLabelsToMetricFamily(family *dto.MetricFamily, vm TartVM) *dto.MetricFamily {
	newFamily := &dto.MetricFamily{
		Name: family.Name,
		Help: family.Help,
		Type: family.Type,
	}

	runnerNameLabel := &dto.LabelPair{
		Name:  &[]string{"runner_name"}[0],
		Value: &vm.RunnerName,
	}
	runnerIDLabel := &dto.LabelPair{
		Name:  &[]string{"runner_id"}[0],
		Value: &vm.RunnerID,
	}

	for _, metric := range family.Metric {
		newMetric := &dto.Metric{}

		// Copy existing labels and add Runner labels
		newMetric.Label = append([]*dto.LabelPair{}, metric.Label...)
		newMetric.Label = append(newMetric.Label, runnerNameLabel, runnerIDLabel)

		// Copy the metric value (counter, gauge, histogram, etc.)
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			newMetric.Counter = metric.Counter
		case dto.MetricType_GAUGE:
			newMetric.Gauge = metric.Gauge
		case dto.MetricType_SUMMARY:
			newMetric.Summary = metric.Summary
		case dto.MetricType_HISTOGRAM:
			newMetric.Histogram = metric.Histogram
		case dto.MetricType_UNTYPED:
			newMetric.Untyped = metric.Untyped
		}

		newMetric.TimestampMs = metric.TimestampMs

		newFamily.Metric = append(newFamily.Metric, newMetric)
	}

	return newFamily
}

// mergeMetricFamilies combines metric families with the same name
func (ma *MetricsAggregator) mergeMetricFamilies(families []*dto.MetricFamily) []*dto.MetricFamily {
	familyMap := make(map[string]*dto.MetricFamily)

	for _, family := range families {
		name := family.GetName()
		if existing, exists := familyMap[name]; exists {
			existing.Metric = append(existing.Metric, family.Metric...)
		} else {
			familyMap[name] = &dto.MetricFamily{
				Name:   family.Name,
				Help:   family.Help,
				Type:   family.Type,
				Metric: append([]*dto.Metric{}, family.Metric...),
			}
		}
	}

	result := make([]*dto.MetricFamily, 0, len(familyMap))
	for _, family := range familyMap {
		result = append(result, family)
	}

	return result
}

// formatMetricsAsPrometheusText converts metric families back to Prometheus text format
func (ma *MetricsAggregator) formatMetricsAsPrometheusText(families []*dto.MetricFamily) (string, error) {
	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))

	for _, family := range families {
		if err := encoder.Encode(family); err != nil {
			return "", fmt.Errorf("failed to encode metric family %s: %w", family.GetName(), err)
		}
	}

	return buf.String(), nil
}
