package metrics_aggregator

//
//import (
//	"context"
//	"fmt"
//	"github.com/prometheus/client_golang/prometheus"
//	dto "github.com/prometheus/client_model/go"
//	"net/http"
//	"net/http/httptest"
//	"testing"
//	"time"
//
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/mock"
//	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
//	tarter "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/rest"
//	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
//)
//
//type TarterClientInterface interface {
//	GetActiveRunners(ctx context.Context) ([]types.RunnerStateList, error)
//	GetAllRunners(ctx context.Context) ([]types.RunnerStateList, error)
//}
//
//type TartClientInterface interface {
//	GetTartVMIP(vmName string) (string, error)
//}
//
//type MockTarterClient struct {
//	mock.Mock
//}
//
//func (m *MockTarterClient) GetActiveRunners(ctx context.Context) ([]types.RunnerStateList, error) {
//	args := m.Called(ctx)
//	return args.Get(0).([]types.RunnerStateList), args.Error(1)
//}
//
//func (m *MockTarterClient) GetAllRunners(ctx context.Context) ([]types.RunnerStateList, error) {
//	args := m.Called(ctx)
//	return args.Get(0).([]types.RunnerStateList), args.Error(1)
//}
//
//type MockTartClient struct {
//	mock.Mock
//}
//
//func (m *MockTartClient) GetTartVMIP(vmName string) (string, error) {
//	args := m.Called(vmName)
//	return args.String(0), args.Error(1)
//}
//
//func TestScrapeTartVMMetrics(t *testing.T) {
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		fmt.Fprintln(w, `# HELP go_goroutines Number of goroutines
//# TYPE go_goroutines gauge
//go_goroutines 33
//# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.
//# TYPE process_cpu_seconds_total counter
//process_cpu_seconds_total 0.34`)
//	}))
//	defer server.Close()
//
//	// Extract port from the test server
//	port := server.URL[len("http://127.0.0.1:"):]
//
//	mockTarterClient := new(MockTarterClient)
//	mockTartClient := new(MockTartClient)
//
//	ma := &MetricsAggregator{
//		tarterClient:  mockTarterClient,
//		tartClient:    mockTartClient,
//		cache:         NewCache(5),
//		scrapePort:    port,
//		scrapeTimeout: scrapeTimeout,
//	}
//
//	vm := TartVM{
//		Name:       "test-vm",
//		RunnerName: "test-runner",
//		RunnerID:   "123",
//		IP:         "127.0.0.1",
//	}
//
//	families, err := ma.scrapeTartVMMetrics(vm)
//
//	assert.NoError(t, err)
//	assert.Len(t, families, 2)
//
//	assert.Equal(t, "go_goroutines", families[0].GetName())
//	assert.Equal(t, "process_cpu_seconds_total", families[1].GetName())
//}
//
//func TestScrapeTartVMMetricsServerError(t *testing.T) {
//	// Create a test server that responds with error
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(http.StatusInternalServerError)
//	}))
//	defer server.Close()
//
//	port := server.URL[len("http://127.0.0.1:"):]
//
//	ma := &MetricsAggregator{
//		cache:         NewCache(5),
//		scrapePort:    port,
//		scrapeTimeout: scrapeTimeout,
//	}
//
//	vm := TartVM{
//		Name: "test-vm",
//		IP:   "127.0.0.1",
//	}
//
//	_, err := ma.scrapeTartVMMetrics(vm)
//
//	// Should return error for non-200 response
//	assert.Error(t, err)
//}
//
//func TestGetRunningVMs(t *testing.T) {
//	mockTarterClient := new(MockTarterClient)
//	mockTartClient := new(MockTartClient)
//
//	ma := &MetricsAggregator{
//		tarterClient:  mockTarterClient,
//		tartClient:    mockTartClient,
//		cache:         NewCache(5),
//		scrapeTimeout: 1 * time.Second,
//	}
//
//	// Setup mocks
//	runners := []types.RunnerStateList{
//		{ID: "1", TartVMName: "vm1", GhaRunnerName: "runner1"},
//		{ID: "2", TartVMName: "vm2", GhaRunnerName: "runner2"},
//	}
//
//	mockTarterClient.On("GetActiveRunners", mock.Anything).Return(runners, nil)
//	mockTartClient.On("GetTartVMIP", "vm1").Return("192.168.1.1", nil)
//	mockTartClient.On("GetTartVMIP", "vm2").Return("192.168.1.2", nil)
//
//	// Call the method
//	vms, err := ma.getRunningVMs()
//
//	// Assertions
//	assert.NoError(t, err)
//	assert.Len(t, vms, 2)
//	assert.Equal(t, "vm1", vms[0].Name)
//	assert.Equal(t, "192.168.1.1", vms[0].IP)
//	assert.Equal(t, "runner1", vms[0].RunnerName)
//}
//
//func TestGetTartVMIPs(t *testing.T) {
//	mockTartClient := new(MockTartClient)
//
//	ma := &MetricsAggregator{
//		tartClient: mockTartClient,
//		cache:      NewCache(5),
//	}
//
//	// Setup mock to be called only once (cache should prevent second call)
//	mockTartClient.On("GetTartVMIP", "vm1").Return("192.168.1.1", nil).Once()
//
//	// First call should hit the client
//	ip, err := ma.getTartVMIPs("vm1")
//	assert.NoError(t, err)
//	assert.Equal(t, "192.168.1.1", ip)
//
//	// Second call should use the cache
//	ip, err = ma.getTartVMIPs("vm1")
//	assert.NoError(t, err)
//	assert.Equal(t, "192.168.1.1", ip)
//
//	// Verify expectations
//	mockTartClient.AssertExpectations(t)
//}
//
//func TestAddRunnerLabelsToMetricFamily(t *testing.T) {
//	ma := &MetricsAggregator{}
//
//	// Create a sample metric family
//	name := "test_metric"
//	help := "Test help text"
//	metricType := dto.MetricType_GAUGE
//	value := float64(42)
//
//	// Create a label pair
//	labelName := "instance"
//	labelValue := "localhost:8080"
//
//	metricFamily := &dto.MetricFamily{
//		Name: &name,
//		Help: &help,
//		Type: &metricType,
//		Metric: []*dto.Metric{
//			{
//				Label: []*dto.LabelPair{
//					{
//						Name:  &labelName,
//						Value: &labelValue,
//					},
//				},
//				Gauge: &dto.Gauge{
//					Value: &value,
//				},
//			},
//		},
//	}
//
//	vm := TartVM{
//		Name:       "test-vm",
//		RunnerName: "test-runner",
//		RunnerID:   "123",
//	}
//
//	// Add runner labels
//	newFamily := ma.addRunnerLabelsToMetricFamily(metricFamily, vm)
//
//	// Check that original labels are preserved
//	assert.Len(t, newFamily.Metric[0].Label, 3)
//	assert.Equal(t, "instance", *newFamily.Metric[0].Label[0].Name)
//
//	// Check that runner labels were added
//	assert.Equal(t, "runner_name", *newFamily.Metric[0].Label[1].Name)
//	assert.Equal(t, "test-runner", *newFamily.Metric[0].Label[1].Value)
//	assert.Equal(t, "runner_id", *newFamily.Metric[0].Label[2].Name)
//	assert.Equal(t, "123", *newFamily.Metric[0].Label[2].Value)
//
//	// Check that metric value was preserved
//	assert.Equal(t, float64(42), *newFamily.Metric[0].Gauge.Value)
//}
//
//func TestMergeMetricFamilies(t *testing.T) {
//	ma := &MetricsAggregator{}
//
//	name1 := "metric1"
//	name2 := "metric2"
//	help := "Help text"
//	metricType := dto.MetricType_GAUGE
//
//	// Create two families with the same name
//	family1 := &dto.MetricFamily{
//		Name:   &name1,
//		Help:   &help,
//		Type:   &metricType,
//		Metric: []*dto.Metric{{}, {}}, // 2 metrics
//	}
//
//	family2 := &dto.MetricFamily{
//		Name:   &name1,
//		Help:   &help,
//		Type:   &metricType,
//		Metric: []*dto.Metric{{}}, // 1 metric
//	}
//
//	// Create a third family with different name
//	family3 := &dto.MetricFamily{
//		Name:   &name2,
//		Help:   &help,
//		Type:   &metricType,
//		Metric: []*dto.Metric{{}}, // 1 metric
//	}
//
//	// Merge them
//	merged := ma.mergeMetricFamilies([]*dto.MetricFamily{family1, family2, family3})
//
//	// Should have 2 unique family names
//	assert.Len(t, merged, 2)
//
//	// Find the merged family by name
//	var mergedFamily1 *dto.MetricFamily
//	var mergedFamily2 *dto.MetricFamily
//
//	for _, f := range merged {
//		if f.GetName() == name1 {
//			mergedFamily1 = f
//		} else if f.GetName() == name2 {
//			mergedFamily2 = f
//		}
//	}
//
//	// Check that metrics were merged
//	assert.NotNil(t, mergedFamily1)
//	assert.Len(t, mergedFamily1.Metric, 3) // 2 + 1 metrics
//
//	assert.NotNil(t, mergedFamily2)
//	assert.Len(t, mergedFamily2.Metric, 1)
//}
//
//func TestCollectRunnersMetrics(t *testing.T) {
//	// Create a test server for metrics
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		fmt.Fprintln(w, `# HELP go_goroutines Number of goroutines
//# TYPE go_goroutines gauge
//go_goroutines 33`)
//	}))
//	defer server.Close()
//
//	// Extract port from the test server
//	port := server.URL[len("http://127.0.0.1:"):]
//
//	mockTarterClient := new(MockTarterClient)
//	mockTartClient := new(MockTartClient)
//
//	ma := &MetricsAggregator{
//		tarterClient:  mockTarterClient,
//		tartClient:    mockTartClient,
//		cache:         NewCache(5),
//		scrapePort:    port,
//		scrapeTimeout: 1 * time.Second,
//		aggregatorMetrics: &AggregatorOwnMetrics{
//			exporterInfo:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_info"}, []string{"version"}),
//			lastScrapeTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_timestamp"}),
//			totalVMCount:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_vm_count"}),
//			successfulScrapes:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_scrapes"}),
//		},
//	}
//
//	// Setup mocks
//	runners := []types.RunnerStateList{
//		{ID: "1", TartVMName: "vm1", GhaRunnerName: "runner1"},
//	}
//
//	mockTarterClient.On("GetActiveRunners", mock.Anything).Return(runners, nil)
//	mockTartClient.On("GetTartVMIP", "vm1").Return("127.0.0.1", nil)
//
//	// Call the method
//	metrics, err := ma.CollectRunnersMetrics()
//
//	// Assertions
//	assert.NoError(t, err)
//	assert.Contains(t, metrics, "go_goroutines")
//	assert.Contains(t, metrics, "runner_name=\"runner1\"")
//	assert.Contains(t, metrics, "runner_id=\"1\"")
//}
//
//func TestCollectRunnersMetricsNoVMs(t *testing.T) {
//	mockTarterClient := new(MockTarterClient)
//
//	ma := &MetricsAggregator{
//		tarterClient:  mockTarterClient,
//		scrapeTimeout: 1 * time.Second,
//	}
//
//	// Setup mock to return empty list
//	mockTarterClient.On("GetActiveRunners", mock.Anything).Return([]types.RunnerStateList{}, nil)
//
//	// Call the method
//	metrics, err := ma.CollectRunnersMetrics()
//
//	// Assertions
//	assert.NoError(t, err)
//	assert.Contains(t, metrics, "# No running Tart VMs found")
//}
