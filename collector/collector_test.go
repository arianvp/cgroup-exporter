package collector

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

//go:embed fixtures/cgroup
var cgroup embed.FS

func TestCanCollectWholeCgroupTree(t *testing.T) {
	c := New(cgroup, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()
	for range metrics {
	}
	// TODO How to make sure no errors were logged.
}

/*(func FuzzCgroupTree(f *testing.F) {
	fs.WalkDir(cgroup, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		buf, err := fs.ReadFile(cgroup, path)
		if err != nil {
			return err
		}
		f.Add(path, buf)
		return nil
	})
	f.Fuzz(func(t *testing.T, path string, content []byte) {
		c := New(fstest.MapFS{
			path: &fstest.MapFile{Data: content},
		})
		metrics := make(chan prometheus.Metric)
		go func() {
			defer close(metrics)
			c.Collect(metrics)
		}()
		<-metrics
	})
}*/

func TestSkipsMaxMemoryMax(t *testing.T) {
	c := New(fstest.MapFS{
		"system.slice/memory.max": &fstest.MapFile{Data: []byte("max")},
	}, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()
	metric := <-metrics
	if metric != nil {
		t.Error("expected nil metric")
	}
}

func TestCollectsOpenFDsFromProcfs(t *testing.T) {
	cgroupfs := fstest.MapFS{
		"system.slice/worker.service/cgroup.procs": &fstest.MapFile{Data: []byte("123\n123\n456\n789\n")},
	}
	procfs := fstest.MapFS{
		"123/fd/0": &fstest.MapFile{},
		"123/fd/1": &fstest.MapFile{},
		"456/fd/0": &fstest.MapFile{},
		"456/fd/1": &fstest.MapFile{},
		"456/fd/2": &fstest.MapFile{},
	}

	c := NewWithProcfs(cgroupfs, procfs, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()

	metric := <-metrics
	if metric == nil {
		t.Fatal("expected metric")
	}

	dto := new(io_prometheus_client.Metric)
	if err := metric.Write(dto); err != nil {
		t.Fatal(err)
	}
	if dto.Gauge == nil || dto.Gauge.Value == nil {
		t.Fatal("expected gauge metric")
	}
	if *dto.Gauge.Value != 5 {
		t.Fatalf("expected 5 open fds, got %f", *dto.Gauge.Value)
	}

	var foundCgroup bool
	for _, l := range dto.Label {
		if *l.Name == "cgroup" {
			foundCgroup = true
			if *l.Value != "system.slice/worker.service" {
				t.Fatalf("expected cgroup label system.slice/worker.service, got %q", *l.Value)
			}
		}
	}
	if !foundCgroup {
		t.Fatal("expected cgroup label")
	}
}

func TestReadPIDsDeDuplicatesAndSkipsBlanks(t *testing.T) {
	pids, err := readPIDs(strings.NewReader("123\n\n123\n456\n"))
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"123", "456"}
	if len(pids) != len(expected) {
		t.Fatalf("expected %d pids, got %d", len(expected), len(pids))
	}
	for i := range expected {
		if pids[i] != expected[i] {
			t.Fatalf("expected pid %q at index %d, got %q", expected[i], i, pids[i])
		}
	}
}

type permissionDeniedFS struct{}

func (permissionDeniedFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrPermission
}

func TestSkipsOpenFDMetricWhenProcfsIsNotPermitted(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	cgroupfs := fstest.MapFS{
		"system.slice/worker.service/cgroup.procs": &fstest.MapFile{Data: []byte("123\n")},
	}

	c := NewWithProcfs(cgroupfs, permissionDeniedFS{}, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()

	metric, ok := <-metrics
	if ok || metric != nil {
		t.Fatal("expected open fd metric to be skipped when procfs is not permitted")
	}
	if logs.Len() != 0 {
		t.Fatalf("expected no error logs, got %q", logs.String())
	}
}

func TestParsesOtherMemory(t *testing.T) {
	mapfs := fstest.MapFS{
		"system.slice/memory.min": &fstest.MapFile{Data: []byte("1\n")},
	}

	c := New(mapfs, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()

	metric := <-metrics
	if metric == nil {
		t.Error("expected metric")
	}
	dto := new(io_prometheus_client.Metric)
	metric.Write(dto)

	if *dto.Gauge.Value != 1 {
		t.Errorf("expected 1 got %f", *dto.Gauge.Value)
	}

}

func TestParsesPressure(t *testing.T) {

	pressure := `some avg10=0.08 avg60=0.03 avg300=0.06 total=7113021
full avg10=0.00 avg60=0.00 avg300=0.00 total=0
`
	mapfs := fstest.MapFS{
		"system.slice/memory.pressure": &fstest.MapFile{Data: []byte(pressure)},
	}
	c := New(mapfs, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()

	metric := <-metrics
	if metric == nil {
		t.Fatal("expected metric")
	}
	dto := new(io_prometheus_client.Metric)
	metric.Write(dto)

	for _, l := range dto.Label {
		if *l.Name == "cgroup" {
			if *l.Value != "system.slice" {
				t.Errorf("expected system.slice got %s", *l.Value)
			}
		}
	}

	if *dto.Counter.Value != 7.113021 {
		t.Errorf("expected 7.113021 got %f", *dto.Counter.Value)
	}

	metric = <-metrics
	if metric == nil {
		t.Fatal("expected metric")
	}
	dto = new(io_prometheus_client.Metric)
	metric.Write(dto)
	if *dto.Counter.Value != 0 {
		t.Errorf("expected 0 got %f", *dto.Counter.Value)
	}

}

func TestVisitNestedKeyed(t *testing.T) {
	pressure := `some avg10=0.08 avg60=0.03 avg300=0.06 total=7113021
full avg10=0.00 avg60=0.00 avg300=0.00 total=0
`
	r := strings.NewReader(pressure)

	err := visitNestedKeyed(r, func(n string) (kvVisitor, error) {
		return func(k, v string) error {
			switch n {
			case "some":
				if k == "avg10" && v != "0.08" {
					return errors.New("expected 0.08")
				}
				if k == "avg60" && v != "0.03" {
					return errors.New("expected 0.03")
				}
				if k == "avg300" && v != "0.06" {
					return errors.New("expected 0.06")
				}
				if k == "total" && v != "7113021" {
					return errors.New("expected 7113021")
				}
			case "full":
				if k == "avg10" && v != "0.00" {
					return errors.New("expected 0.00")
				}
				if k == "avg60" && v != "0.00" {
					return errors.New("expected 0.00")
				}
				if k == "avg300" && v != "0.00" {
					return errors.New("expected 0.00")
				}
				if k == "total" && v != "0" {
					return errors.New("expected 0")
				}
			default:
				return errors.New("unexpected key")
			}
			return nil

		}, nil
	})
	if err != nil {
		t.Error(err)
	}
}

func TestVisitNestedKeyedSkipsNilVisitor(t *testing.T) {
	input := `skip this=entry
keep value=42
`

	seen := false
	err := visitNestedKeyed(strings.NewReader(input), func(n string) (kvVisitor, error) {
		if n == "skip" {
			return nil, nil
		}
		return func(k, v string) error {
			if n == "keep" && k == "value" && v == "42" {
				seen = true
			}
			return nil
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("expected keep/value=42 to be visited")
	}
}

type spyfs struct {
	t *testing.T
	fs.FS
}

func (s *spyfs) Open(name string) (fs.File, error) {
	s.t.Log("opening", name)
	if name == "system.slice/lol.doesntaccessme" {
		s.t.Error("should not open file it doesn't have a description for")
	}
	return s.FS.Open(name)
}

func TestDoesntOpenFilesThatItDoesntHaveDescriptionsFor(t *testing.T) {
	fs := fstest.MapFS{
		"system.slice/memory.pressure":    &fstest.MapFile{Data: []byte("some avg10=0.08 avg60=0.03 avg300=0.06 total=7113021\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n")},
		"system.slice/lol.doesntaccessme": &fstest.MapFile{Data: []byte("some avg10=0.08 avg60=0.03 avg300=0.06 total=7113021\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n")},
	}
	spy := &spyfs{t: t, FS: fs}

	c := New(spy, "")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()
	<-metrics
}

func TestFindFilesInLeafDirectoriesRoot(t *testing.T) {
	mapfs := fstest.MapFS{
		"memory.current": &fstest.MapFile{Data: []byte("1234\n")},
		"cpu.stat":       &fstest.MapFile{Data: []byte("usage_usec 100\n")},
	}
	var paths []string
	err := FindFilesInLeafDirectories(mapfs, ".", func(filePath string, entry fs.DirEntry) error {
		paths = append(paths, filePath)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("expected files in root directory")
	}
	for _, p := range paths {
		if !fs.ValidPath(p) {
			t.Errorf("path %q is not a valid fs.FS path", p)
		}
	}
}

func TestCollectsFromRootCgroup(t *testing.T) {
	mapfs := fstest.MapFS{
		"memory.current": &fstest.MapFile{Data: []byte("1234\n")},
	}
	c := New(mapfs, ".")
	metrics := make(chan prometheus.Metric)
	go func() {
		defer close(metrics)
		c.Collect(metrics)
	}()
	metric := <-metrics
	if metric == nil {
		t.Fatal("expected metric from root cgroup")
	}
	dto := new(io_prometheus_client.Metric)
	if err := metric.Write(dto); err != nil {
		t.Fatal(err)
	}
	if *dto.Gauge.Value != 1234 {
		t.Errorf("expected 1234 got %f", *dto.Gauge.Value)
	}
	for _, l := range dto.Label {
		if *l.Name == "cgroup" && *l.Value != "." {
			t.Errorf("expected cgroup label '.' got %q", *l.Value)
		}
	}
}

func TestCanParseRootIOStart(t *testing.T) {
	iostat := `
7:7 
7:6 
7:5 
7:4 
7:3 
7:2 
7:1 
7:0 
254:0 rbytes=4235943936 wbytes=37844828160 rios=72223 wios=2392288 dbytes=0 dios=0
259:0 rbytes=4249748992 wbytes=37844833792 rios=77099 wios=2067962 dbytes=1620828160 dios=9
`
	mapfs := fstest.MapFS{
		"io.stat": &fstest.MapFile{Data: []byte(iostat)},
	}
	c := New(mapfs, "").(*cgroupCollector)
	ms := make(chan prometheus.Metric)
	go func() {
		defer close(ms)
		if err := collectIOStat(strings.NewReader(iostat), ".", c.multipleCollectors["io.stat"].descs, ms); err != nil {
			t.Error(err)
		}

	}()

	for m := range ms {
		dto := new(io_prometheus_client.Metric)
		m.Write(dto)
		for _, l := range dto.Label {
			if *l.Name == "cgroup" {
				if *l.Value != "." {
					t.Errorf("expected . got %s", *l.Value)
				}
			}
		}
	}

}

func TestIOStatSkipsInvalidDevices(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	iostat := `(unknown) rbytes=0 wbytes=0 rios=6 wios=0 dbytes=0 dios=0
259:3 rbytes=241664 wbytes=0 rios=6 wios=0 dbytes=0 dios=0
(unknown) rbytes=0 wbytes=0 rios=13 wios=0 dbytes=0 dios=0
259:2 rbytes=581632 wbytes=0 rios=13 wios=0 dbytes=0 dios=0
`
	c := New(fstest.MapFS{}, "").(*cgroupCollector)
	ms := make(chan prometheus.Metric)
	go func() {
		defer close(ms)
		if err := collectIOStat(strings.NewReader(iostat), "init.scope", c.multipleCollectors["io.stat"].descs, ms); err != nil {
			t.Error(err)
		}
	}()

	var metricCount int
	for m := range ms {
		metricCount++
		dto := new(io_prometheus_client.Metric)
		if err := m.Write(dto); err != nil {
			t.Fatal(err)
		}
		for _, l := range dto.Label {
			if *l.Name == "device" && *l.Value == "(unknown)" {
				t.Fatal("unexpected metric emitted for invalid device")
			}
		}
	}

	if metricCount != 12 {
		t.Fatalf("expected 12 metrics from 2 valid devices, got %d", metricCount)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "skipping io.stat entry with invalid device") {
		t.Fatalf("expected warning log, got %q", logOutput)
	}
	if strings.Count(logOutput, "device=(unknown)") != 1 {
		t.Fatalf("expected one warning for invalid device, got %q", logOutput)
	}
}

func TestCanParseNumaStat(t *testing.T) {
	numastat := `anon N0=3286069248
file N0=4915474432 N1=12345678
kernel_stack N0=19283968
`
	mapfs := fstest.MapFS{
		"memory.numa_stat": &fstest.MapFile{Data: []byte(numastat)},
	}
	c := New(mapfs, "").(*cgroupCollector)
	ms := make(chan prometheus.Metric)
	go func() {
		defer close(ms)
		if err := collectNumaStat(strings.NewReader(numastat), "test.slice", c.multipleCollectors["memory.numa_stat"].descs, ms); err != nil {
			t.Error(err)
		}
	}()

	expectedValues := map[string]map[string]float64{
		"cgroup_memory_numa_anon_bytes":         {"0": 3286069248},
		"cgroup_memory_numa_file_bytes":         {"0": 4915474432, "1": 12345678},
		"cgroup_memory_numa_kernel_stack_bytes": {"0": 19283968},
	}
	foundMetrics := make(map[string]map[string]bool)

	// Regex to extract metric name from Desc string
	fqNameRe := regexp.MustCompile(`fqName:\s*"([^"]+)"`)

	for m := range ms {
		dto := new(io_prometheus_client.Metric)
		m.Write(dto)

		// Extract metric name from Desc string
		descStr := m.Desc().String()
		matches := fqNameRe.FindStringSubmatch(descStr)
		if len(matches) < 2 {
			t.Errorf("could not extract metric name from %s", descStr)
			continue
		}
		metricName := matches[1]

		var numaNode, cgroup string
		for _, l := range dto.Label {
			if *l.Name == "numa_node" {
				numaNode = *l.Value
			}
			if *l.Name == "cgroup" {
				cgroup = *l.Value
			}
		}

		if cgroup != "test.slice" {
			t.Errorf("expected cgroup=test.slice, got %s", cgroup)
		}

		if expectedNodes, ok := expectedValues[metricName]; ok {
			if expectedValue, ok := expectedNodes[numaNode]; ok {
				if actualValue := *dto.Gauge.Value; actualValue != expectedValue {
					t.Errorf("metric %s for %s: expected %f, got %f", metricName, numaNode, expectedValue, actualValue)
				}
				if foundMetrics[metricName] == nil {
					foundMetrics[metricName] = make(map[string]bool)
				}
				foundMetrics[metricName][numaNode] = true
			} else {
				t.Errorf("unexpected numa_node %s for metric %s", numaNode, metricName)
			}
		} else {
			t.Errorf("unexpected metric %s", metricName)
		}
	}

	for metricName, nodes := range expectedValues {
		for node := range nodes {
			if !foundMetrics[metricName][node] {
				t.Errorf("expected metric %s for %s not found", metricName, node)
			}
		}
	}
}

func TestCanParseNumaStatWithUnknownEntry(t *testing.T) {
	numastat := `unknown N0=1234
anon N0=5678
`

	c := New(fstest.MapFS{}, "").(*cgroupCollector)
	ms := make(chan prometheus.Metric, 10)
	if err := collectNumaStat(strings.NewReader(numastat), "test.slice", c.multipleCollectors["memory.numa_stat"].descs, ms); err != nil {
		t.Fatal(err)
	}
	close(ms)

	found := false
	for m := range ms {
		desc := m.Desc().String()
		if strings.Contains(desc, `fqName: "cgroup_memory_numa_anon_bytes"`) {
			found = true
			dto := new(io_prometheus_client.Metric)
			m.Write(dto)
			if *dto.Gauge.Value != 5678 {
				t.Fatalf("expected 5678 got %f", *dto.Gauge.Value)
			}
		}
	}

	if !found {
		t.Fatal("expected anon metric to be collected")
	}
}
