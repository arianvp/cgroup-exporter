package collector

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const LinuxCGroupsDir = "/sys/fs/cgroup"

type cgroupCollector struct {
	fs                 fs.FS
	glob               string
	singleCollectors   map[string]collector
	multipleCollectors map[string]multipleCollector
}

type collector struct {
	desc    *prometheus.Desc
	collect collectFunc
}

type desc struct {
	desc     *prometheus.Desc
	modifier func(float64) float64
}

type multipleCollector struct {
	descs   map[string]desc
	collect collectMultipleFunc
}

type collectFunc func(f io.Reader, path string, desc *prometheus.Desc, m chan<- prometheus.Metric) error
type collectMultipleFunc func(f io.Reader, path string, desc map[string]desc, m chan<- prometheus.Metric) error

func microSecondsToSeconds(microseconds float64) float64 {
	return microseconds / 1e6
}

func NewCollector(options ...Option) prometheus.Collector {
	result := &cgroupCollector{
		fs:                 os.DirFS(LinuxCGroupsDir),
		glob:               "*",
		singleCollectors:   map[string]collector{},
		multipleCollectors: map[string]multipleCollector{},
	}

	for _, o := range options {
		o(result)
	}

	return result
}

func New(fs fs.FS, glob string) prometheus.Collector {
	return NewCollector(
		WithCGroupFS(fs),
		WithCGroupGlob(glob),
		WithMemoryBaseCollectors(),
		WithMemorySwapCollectors(),
		WithMemoryZSwapCollectors(),
		WithMemoryNUMAStatCollectors(),
		WithMemoryStatCollectors(),
		WithMemoryEventsCollectors(),
		WithMemoryPressureCollectors(),
		WithCPUStatCollectors(),
		WithCPUPressureCollectors(),
		WithIOStatCollectors(),
		WithIOPressureCollectors(),
		WithPIDBaseCollectors(),
		WithPIDEventsCollectors(),
	)
}

// FindFilesInLeafDirectories finds and processes all files in leaf directories
func FindFilesInLeafDirectories(fsys fs.FS, root string, process func(filePath string, entry fs.DirEntry) error) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Check if the directory is a leaf directory
			isLeaf := true
			subDirEntries, readErr := fs.ReadDir(fsys, path)
			if readErr != nil {
				return readErr
			}

			// Determine if the directory has any subdirectories
			for _, entry := range subDirEntries {
				if entry.IsDir() {
					isLeaf = false
					break
				}
			}

			// If it's a leaf directory, process its files
			if isLeaf {
				for _, entry := range subDirEntries {
					if !entry.IsDir() {
						filePath := pathpkg.Join(path, entry.Name())
						if err := process(filePath, entry); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	})
}

// Collect implements prometheus.Collector.
func (c *cgroupCollector) Collect(m chan<- prometheus.Metric) {
	matches, err := fs.Glob(c.fs, c.glob)
	if err != nil {
		slog.Error("failed to glob cgroups", "error", err)
	}
	for _, match := range matches {
		if err := FindFilesInLeafDirectories(c.fs, match, func(path string, d fs.DirEntry) error {
			name := d.Name()

			if col, ok := c.singleCollectors[name]; ok {
				f, err := c.fs.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open file %q: %w", path, err)
				}
				defer f.Close()
				if err := col.collect(f, pathpkg.Dir(path), col.desc, m); err != nil {
					slog.Error("failed to collect cgroup", "error", err)
				}
			}
			if col, ok := c.multipleCollectors[name]; ok {
				f, err := c.fs.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open file %q: %w", path, err)
				}
				defer f.Close()
				if err := col.collect(f, pathpkg.Dir(path), col.descs, m); err != nil {
					slog.Error("failed to collect cgroup", "error", err)
				}
			}

			// TODO: refactor stuff so this is generic
			if name == "io.stat" {
			}

			return nil
		}); err != nil {
			slog.Error("failed to walk cgroup", "error", err)
		}
	}
}

func collectIOStat(f io.Reader, path string, descs map[string]desc, m chan<- prometheus.Metric) error {
	warnedInvalidDevices := map[string]struct{}{}
	return visitNestedKeyed(f, func(n string) (kvVisitor, error) {
		if !isBlockDeviceID(n) {
			if _, warned := warnedInvalidDevices[n]; !warned {
				slog.Warn("skipping io.stat entry with invalid device", "device", n, "cgroup", path)
				warnedInvalidDevices[n] = struct{}{}
			}
			return nil, nil
		}
		device := n
		return func(k, v string) error {
			desc, ok := descs[k]
			if !ok {
				return nil
			}
			value, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("failed to parse value %q: %w", v, err)
			}
			m <- prometheus.MustNewConstMetric(desc.desc, prometheus.CounterValue, value, device, path)
			return nil
		}, nil
	})
}

func isBlockDeviceID(device string) bool {
	major, minor, ok := strings.Cut(device, ":")
	if !ok || major == "" || minor == "" {
		return false
	}

	if _, err := strconv.ParseUint(major, 10, 32); err != nil {
		return false
	}
	if _, err := strconv.ParseUint(minor, 10, 32); err != nil {
		return false
	}

	return true
}

func collectNumaStat(f io.Reader, path string, descs map[string]desc, m chan<- prometheus.Metric) error {
	return visitNestedKeyed(f, func(statName string) (kvVisitor, error) {
		desc, ok := descs[statName]
		if !ok {
			return nil, nil
		}
		return func(node, value string) error {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil
			}
			// Remove 'N' prefix from node name (e.g., N0 -> 0) to match node_exporter format
			node = strings.TrimPrefix(node, "N")
			m <- prometheus.MustNewConstMetric(desc.desc, prometheus.GaugeValue, v, node, path)
			return nil
		}, nil
	})
}

func collectSingleValue(valueType prometheus.ValueType) collectFunc {
	return func(f io.Reader, path string, desc *prometheus.Desc, m chan<- prometheus.Metric) error {

		var val string
		if _, err := fmt.Fscanf(f, "%s", &val); err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
		if val == "max" {
			return nil
		}
		var value float64
		value, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("failed to parse value %q: %w", val, err)
		}
		m <- prometheus.MustNewConstMetric(desc, valueType, value, path)
		return nil
	}
}

type kvVisitor func(k, v string) error
type entryVisitor func(n string) (kvVisitor, error)

// visitNestedKeyed parses r into nested key-values. The format is expected to be:
//
//	KEY0 SUB_KEY0=VAL00 SUB_KEY1=VAL01\n
//	KEY1 SUB_KEY0=VAL10 SUB_KEY1=VAL11\n
//	...
func visitNestedKeyed(r io.Reader, visitEntry entryVisitor) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		k := fields[0]
		vs := fields[1:]
		visitKV, err := visitEntry(k)
		if err != nil {
			return err
		}
		if visitKV == nil {
			continue
		}

		for _, v := range vs {
			kv := strings.Split(v, "=")
			if len(kv) == 0 {
				// some entries might not have values
				return nil
			}
			if len(kv) != 2 {
				return fmt.Errorf("invalid key-value pair %q %q, %d", k, v, len(kv))
			}
			visitKV(kv[0], kv[1])
		}

	}
	return scanner.Err()
}

// visitFlatKeyed parses r into key-values. The format is expected to be:
//
//	KEY0 VAL0\n
//	KEY1 VAL1\n
func visitFlatKeyed(r io.Reader, visitKV kvVisitor) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		kv := strings.Split(line, " ")
		if len(kv) != 2 {
			return fmt.Errorf("invalid key-value pair %q", line)
		}
		if err := visitKV(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// collectFlatKeyed collects a file with multiple key-value pairs.
func collectFlatKeyed(valueType prometheus.ValueType) collectMultipleFunc {
	return func(f io.Reader, path string, descs map[string]desc, m chan<- prometheus.Metric) error {
		return visitFlatKeyed(f, func(k, v string) error {
			desc, ok := descs[k]
			if !ok {
				// silently skip unknown keys
				return nil
			}
			value, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("failed to parse value %q: %w", v, err)
			}
			if desc.modifier != nil {
				value = desc.modifier(value)
			}
			m <- prometheus.MustNewConstMetric(desc.desc, valueType, value, path)
			return nil
		})
	}
}

// collectPressure collects a file with pressure values. Currently only total is collected as the
// other values can easily be derived from the time-series data.
func collectPressure(f io.Reader, path string, descs map[string]desc, m chan<- prometheus.Metric) error {

	return visitNestedKeyed(f, func(n string) (kvVisitor, error) {
		desc, ok := descs[n]
		if !ok {
			return nil, fmt.Errorf("unknown pressure type %q", n)
		}

		return func(k, v string) error {
			if k == "total" {
				value, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return fmt.Errorf("failed to parse value %q: %w", v, err)
				}
				if desc.modifier != nil {
					value = desc.modifier(value)
				}
				m <- prometheus.MustNewConstMetric(desc.desc, prometheus.CounterValue, value, path)
			}
			return nil
		}, nil
	})
}

// Describe implements prometheus.Collector.
func (c *cgroupCollector) Describe(d chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, d)
}

var _ prometheus.Collector = &cgroupCollector{}
