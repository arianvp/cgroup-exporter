package collector

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

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

func New(fs fs.FS, glob string) prometheus.Collector {
	if glob == "" {
		glob = "*"
	}
	return &cgroupCollector{
		fs:   fs,
		glob: glob,
		singleCollectors: map[string]collector{
			"memory.min":     {desc: prometheus.NewDesc("cgroup_memory_min_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.low":     {desc: prometheus.NewDesc("cgroup_memory_low_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.high":    {desc: prometheus.NewDesc("cgroup_memory_high_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.max":     {desc: prometheus.NewDesc("cgroup_memory_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.current": {desc: prometheus.NewDesc("cgroup_memory_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},

			"memory.swap.high":    {desc: prometheus.NewDesc("cgroup_memory_swap_high_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.swap.max":     {desc: prometheus.NewDesc("cgroup_memory_swap_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.swap.current": {desc: prometheus.NewDesc("cgroup_memory_swap_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},

			"memory.zswap.max":     {desc: prometheus.NewDesc("cgroup_memory_zswap_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"memory.zswap.current": {desc: prometheus.NewDesc("cgroup_memory_zswap_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},

			"pids.current": {desc: prometheus.NewDesc("cgroup_pids_current", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
			"pids.max":     {desc: prometheus.NewDesc("cgroup_pids_max", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)},
		},
		multipleCollectors: map[string]multipleCollector{
			// TODO: memory.numastat
			"memory.stat": {
				descs: map[string]desc{
					"anon":                     {desc: prometheus.NewDesc("cgroup_memory_anon_bytes", "Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS)", []string{"cgroup"}, nil)},
					"file":                     {desc: prometheus.NewDesc("cgroup_memory_file_bytes", "Amount of memory used to cache filesystem data, including tmpfs and shared memory.", []string{"cgroup"}, nil)},
					"kernel":                   {desc: prometheus.NewDesc("cgroup_memory_kernel_bytes", "Amount of total kernel memory, including (kernel_stack, pagetables, percpu, vmalloc, slab) in addition to other kernel memory use cases.", []string{"cgroup"}, nil)},
					"kernel_stack":             {desc: prometheus.NewDesc("cgroup_memory_kernel_stack_bytes", "Amount of memory allocated to kernel stacks.", []string{"cgroup"}, nil)},
					"pagetables":               {desc: prometheus.NewDesc("cgroup_memory_pagetables_bytes", "Amount of memory allocated for page tables.", []string{"cgroup"}, nil)},
					"sec_pagetables":           {desc: prometheus.NewDesc("cgroup_memory_sec_pagetables_bytes", "Amount of memory allocated for secondary page tables, this currently includes KVM mmu allocations on x86 and arm64 and IOMMU page tables.", []string{"cgroup"}, nil)},
					"percpu":                   {desc: prometheus.NewDesc("cgroup_memory_percpu_bytes", "Amount of memory used for storing per-cpu kernel data structures.", []string{"cgroup"}, nil)},
					"sock":                     {desc: prometheus.NewDesc("cgroup_memory_sock_bytes", "Amount of memory used in network transmission buffers", []string{"cgroup"}, nil)},
					"vmalloc":                  {desc: prometheus.NewDesc("cgroup_memory_vmalloc_bytes", "Amount of memory used for vmap backed memory.", []string{"cgroup"}, nil)},
					"shmem":                    {desc: prometheus.NewDesc("cgroup_memory_shmem_bytes", "Amount of cached filesystem data that is swap-backed, such as tmpfs, shm segments, shared anonymous mmap()s", []string{"cgroup"}, nil)},
					"zswap":                    {desc: prometheus.NewDesc("cgroup_memory_zswap_bytes", "Amount of memory consumed by the zswap compression backend.", []string{"cgroup"}, nil)},
					"zswapped":                 {desc: prometheus.NewDesc("cgroup_memory_zswapped_bytes", "Amount of application memory swapped out to zswap.", []string{"cgroup"}, nil)},
					"file_mapped":              {desc: prometheus.NewDesc("cgroup_memory_file_mapped_bytes", "Amount of cached filesystem data mapped with mmap()", []string{"cgroup"}, nil)},
					"file_dirty":               {desc: prometheus.NewDesc("cgroup_memory_file_dirty_bytes", "Amount of cached filesystem data that was modified but not yet written back to disk", []string{"cgroup"}, nil)},
					"file_writeback":           {desc: prometheus.NewDesc("cgroup_memory_file_writeback_bytes", "Amount of cached filesystem data that was modified and is currently being written back to disk", []string{"cgroup"}, nil)},
					"swapcached":               {desc: prometheus.NewDesc("cgroup_memory_swapcached_bytes", "Amount of swap cached in memory. The swapcache is accounted against both memory and swap usage.", []string{"cgroup"}, nil)},
					"anon_thp":                 {desc: prometheus.NewDesc("cgroup_memory_anon_thp_bytes", "Amount of memory used in anonymous mappings backed by transparent hugepages", []string{"cgroup"}, nil)},
					"file_thp":                 {desc: prometheus.NewDesc("cgroup_memory_file_thp_bytes", "Amount of cached filesystem data backed by transparent hugepages", []string{"cgroup"}, nil)},
					"shmem_thp":                {desc: prometheus.NewDesc("cgroup_memory_shmem_thp_bytes", "Amount of shm, tmpfs, shared anonymous mmap()s backed by transparent hugepages", []string{"cgroup"}, nil)},
					"inactive_anon":            {desc: prometheus.NewDesc("cgroup_memory_inactive_anon_bytes", "Amount of memory on the inactive anonymous list", []string{"cgroup"}, nil)},
					"active_anon":              {desc: prometheus.NewDesc("cgroup_memory_active_anon_bytes", "Amount of memory on the active anonymous list", []string{"cgroup"}, nil)},
					"inactive_file":            {desc: prometheus.NewDesc("cgroup_memory_inactive_file_bytes", "Amount of memory on the inactive file list", []string{"cgroup"}, nil)},
					"active_file":              {desc: prometheus.NewDesc("cgroup_memory_active_file_bytes", "Amount of memory on the active file list", []string{"cgroup"}, nil)},
					"unevictable":              {desc: prometheus.NewDesc("cgroup_memory_unevictable_bytes", "Amount of memory that cannot be reclaimed", []string{"cgroup"}, nil)},
					"slab_reclaimable":         {desc: prometheus.NewDesc("cgroup_memory_slab_reclaimable_bytes", "Amount of slab memory that might be reclaimed, such as dentries and inodes.", []string{"cgroup"}, nil)},
					"slab_unreclaimable":       {desc: prometheus.NewDesc("cgroup_memory_slab_unreclaimable_bytes", "Amount of slab memory that cannot be reclaimed under memory pressure.", []string{"cgroup"}, nil)},
					"slab":                     {desc: prometheus.NewDesc("cgroup_memory_slab_bytes", "Amount of memory used for storing in-kernel data structures.", []string{"cgroup"}, nil)},
					"workingset_refault_anon":  {desc: prometheus.NewDesc("cgroup_memory_workingset_refault_anon", "Number of refaults of previously evicted anonymous pages.", []string{"cgroup"}, nil)},
					"workingset_refault_file":  {desc: prometheus.NewDesc("cgroup_memory_workingset_refault_file", "Number of refaults of previously evicted file pages.", []string{"cgroup"}, nil)},
					"workingset_activate_anon": {desc: prometheus.NewDesc("cgroup_memory_workingset_activate_anon", "Number of refaulted anonymous pages that were immediately activated.", []string{"cgroup"}, nil)},
					"workingset_activate_file": {desc: prometheus.NewDesc("cgroup_memory_workingset_activate_file", "Number of refaulted file pages that were immediately activated.", []string{"cgroup"}, nil)},
					"workingset_restore_anon":  {desc: prometheus.NewDesc("cgroup_memory_workingset_restore_anon", "Number of restored anonymous pages detected as an active workingset before they got reclaimed.", []string{"cgroup"}, nil)},
					"workingset_restore_file":  {desc: prometheus.NewDesc("cgroup_memory_workingset_restore_file", "Number of restored file pages detected as an active workingset before they got reclaimed.", []string{"cgroup"}, nil)},
					"workingset_nodereclaim":   {desc: prometheus.NewDesc("cgroup_memory_workingset_nodereclaim", "Number of times a shadow node has been reclaimed.", []string{"cgroup"}, nil)},
					"pgscan":                   {desc: prometheus.NewDesc("cgroup_memory_pgscan", "Amount of scanned pages (in an inactive LRU list)", []string{"cgroup"}, nil)},
					"pgsteal":                  {desc: prometheus.NewDesc("cgroup_memory_pgsteal", "Amount of reclaimed pages.", []string{"cgroup"}, nil)},
					"pgscan_kswapd":            {desc: prometheus.NewDesc("cgroup_memory_pgscan_kswapd", "Amount of scanned pages by kswapd (in an inactive LRU list)", []string{"cgroup"}, nil)},
					"pgscan_direct":            {desc: prometheus.NewDesc("cgroup_memory_pgscan_direct", "Amount of scanned pages directly (in an inactive LRU list)", []string{"cgroup"}, nil)},
					"pgscan_khugepaged":        {desc: prometheus.NewDesc("cgroup_memory_pgscan_khugepaged", "Amount of scanned pages by khugepaged (in an inactive LRU list)", []string{"cgroup"}, nil)},
					"pgsteal_kswapd":           {desc: prometheus.NewDesc("cgroup_memory_pgsteal_kswapd", "Amount of reclaimed pages by kswapd", []string{"cgroup"}, nil)},
					"pgsteal_direct":           {desc: prometheus.NewDesc("cgroup_memory_pgsteal_direct", "Amount of reclaimed pages directly", []string{"cgroup"}, nil)},
					"pgsteal_khugepaged":       {desc: prometheus.NewDesc("cgroup_memory_pgsteal_khugepaged", "Amount of reclaimed pages by khugepaged", []string{"cgroup"}, nil)},
					"pgfault":                  {desc: prometheus.NewDesc("cgroup_memory_pgfault", "Total number of page faults incurred.", []string{"cgroup"}, nil)},
					"pgmajfault":               {desc: prometheus.NewDesc("cgroup_memory_pgmajfault", "Number of major page faults incurred.", []string{"cgroup"}, nil)},
					"pgrefill":                 {desc: prometheus.NewDesc("cgroup_memory_pgrefill", "Amount of scanned pages (in an active LRU list).", []string{"cgroup"}, nil)},
					"pgactivate":               {desc: prometheus.NewDesc("cgroup_memory_pgactivate", "Amount of pages moved to the active LRU list.", []string{"cgroup"}, nil)},
					"pgdeactivate":             {desc: prometheus.NewDesc("cgroup_memory_pgdeactivate", "Amount of pages moved to the inactive LRU list.", []string{"cgroup"}, nil)},
					"pglazyfree":               {desc: prometheus.NewDesc("cgroup_memory_pglazyfree", "Amount of pages postponed to be freed under memory pressure.", []string{"cgroup"}, nil)},
					"pglazyfreed":              {desc: prometheus.NewDesc("cgroup_memory_pglazyfreed", "Amount of reclaimed lazyfree pages.", []string{"cgroup"}, nil)},
					"zswpin":                   {desc: prometheus.NewDesc("cgroup_memory_zswpin", "Number of pages moved in to memory from zswap.", []string{"cgroup"}, nil)},
					"zswpout":                  {desc: prometheus.NewDesc("cgroup_memory_zswpout", "Number of pages moved out of memory to zswap.", []string{"cgroup"}, nil)},
					"zswpwb":                   {desc: prometheus.NewDesc("cgroup_memory_zswpwb", "Number of pages written from zswap to swap.", []string{"cgroup"}, nil)},
					"thp_fault_alloc":          {desc: prometheus.NewDesc("cgroup_memory_thp_fault_alloc", "Number of transparent hugepages allocated to satisfy a page fault.", []string{"cgroup"}, nil)},
					"thp_collapse_alloc":       {desc: prometheus.NewDesc("cgroup_memory_thp_collapse_alloc", "Number of transparent hugepages allocated to allow collapsing an existing range of pages.", []string{"cgroup"}, nil)},
					"thp_swpout":               {desc: prometheus.NewDesc("cgroup_memory_thp_swpout", "Number of transparent hugepages which are swapout in one piece without splitting.", []string{"cgroup"}, nil)},
					"thp_swpout_fallback":      {desc: prometheus.NewDesc("cgroup_memory_thp_swpout_fallback", "Number of transparent hugepages split before swapout due to failed allocation of continuous swap space.", []string{"cgroup"}, nil)},
				},
				collect: collectFlatKeyed(prometheus.GaugeValue),
			},
			"memory.events": {descs: map[string]desc{
				"low":            {desc: prometheus.NewDesc("cgroup_memory_events_low_total", "", []string{"cgroup"}, nil)},
				"high":           {desc: prometheus.NewDesc("cgroup_memory_events_high_total", "", []string{"cgroup"}, nil)},
				"max":            {desc: prometheus.NewDesc("cgroup_memory_events_max_total", "", []string{"cgroup"}, nil)},
				"oom":            {desc: prometheus.NewDesc("cgroup_memory_events_oom_total", "", []string{"cgroup"}, nil)},
				"oom_kill":       {desc: prometheus.NewDesc("cgroup_memory_events_oom_kill_total", "", []string{"cgroup"}, nil)},
				"oom_group_kill": {desc: prometheus.NewDesc("cgroup_memory_events_oom_group_kill_total", "", []string{"cgroup"}, nil)},
			}, collect: collectFlatKeyed(prometheus.CounterValue)},
			"memory.pressure": {descs: map[string]desc{
				"some": {desc: prometheus.NewDesc("cgroup_memory_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"full": {desc: prometheus.NewDesc("cgroup_memory_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			}, collect: collectPressure},
			"cpu.pressure": {descs: map[string]desc{
				"some": {desc: prometheus.NewDesc("cgroup_cpu_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"full": {desc: prometheus.NewDesc("cgroup_cpu_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			}, collect: collectPressure},
			"io.pressure": {descs: map[string]desc{
				"some": {desc: prometheus.NewDesc("cgroup_io_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"full": {desc: prometheus.NewDesc("cgroup_io_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			}, collect: collectPressure},
			"cpu.stat": {descs: map[string]desc{
				"usage_usec":                 {desc: prometheus.NewDesc("cgroup_cpu_usage_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"user_usec":                  {desc: prometheus.NewDesc("cgroup_cpu_user_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"system_usec":                {desc: prometheus.NewDesc("cgroup_cpu_system_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"nr_periods":                 {desc: prometheus.NewDesc("cgroup_cpu_periods_total", "", []string{"cgroup"}, nil)},
				"nr_throttled":               {desc: prometheus.NewDesc("cgroup_cpu_throttled_total", "", []string{"cgroup"}, nil)},
				"throttled_usec":             {desc: prometheus.NewDesc("cgroup_cpu_throttled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"nr_bursts":                  {desc: prometheus.NewDesc("cgroup_cpu_bursts_total", "", []string{"cgroup"}, nil)},
				"burst_usec":                 {desc: prometheus.NewDesc("cgroup_cpu_burst_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
				"core_sched.force_idle_usec": {desc: prometheus.NewDesc("cgroup_cpu_core_sched_force_idle_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			}, collect: collectFlatKeyed(prometheus.CounterValue)},
			"io.stat": {descs: map[string]desc{
				"rbytes": {desc: prometheus.NewDesc("cgroup_io_read_bytes_total", "", []string{"device", "cgroup"}, nil)},
				"wbytes": {desc: prometheus.NewDesc("cgroup_io_write_bytes_total", "", []string{"device", "cgroup"}, nil)},
				"dbytes": {desc: prometheus.NewDesc("cgroup_io_discard_bytes_total", "", []string{"device", "cgroup"}, nil)},
				"rios":   {desc: prometheus.NewDesc("cgroup_io_read_operations_total", "", []string{"device", "cgroup"}, nil)},
				"wios":   {desc: prometheus.NewDesc("cgroup_io_write_operations_total", "", []string{"device", "cgroup"}, nil)},
				"dios":   {desc: prometheus.NewDesc("cgroup_io_discard_operations_total", "", []string{"device", "cgroup"}, nil)},
			}, collect: collectIOStat},
			"pids.events": {descs: map[string]desc{
				"max": {desc: prometheus.NewDesc("cgroup_pids_events_max_total", "", []string{"cgroup"}, nil)},
			}, collect: collectFlatKeyed(prometheus.CounterValue)},
		},
	}
}

// Collect implements prometheus.Collector.
func (c *cgroupCollector) Collect(m chan<- prometheus.Metric) {
	matches, err := fs.Glob(c.fs, c.glob)
	if err != nil {
		slog.Error("failed to glob cgroups", "error", err)
	}
	for _, match := range matches {
		if err := fs.WalkDir(c.fs, match, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("failed to walk cgroup: %w", err)
			}
			if d.IsDir() {
				return nil
			}

			name := d.Name()

			if col, ok := c.singleCollectors[name]; ok {
				f, err := c.fs.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open file %q: %w", path, err)
				}
				defer f.Close()
				if err := col.collect(f, filepath.Dir(path), col.desc, m); err != nil {
					slog.Error("failed to collect cgroup", "error", err)
				}
			}
			if col, ok := c.multipleCollectors[name]; ok {
				f, err := c.fs.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open file %q: %w", path, err)
				}
				defer f.Close()
				if err := col.collect(f, filepath.Dir(path), col.descs, m); err != nil {
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
	return visitNestedKeyed(f, func(n string) (kvVisitor, error) {
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
		line := scanner.Text()
		line = strings.TrimSpace(line)
		split := strings.Split(line, " ")
		k := split[0]
		vs := split[1:]
		visitKV, err := visitEntry(k)
		if err != nil {
			return err
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
