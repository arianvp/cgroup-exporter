package collector

import (
	"embed"
	"errors"
	"io/fs"
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
