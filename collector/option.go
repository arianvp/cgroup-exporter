package collector

import (
	"io/fs"

	"github.com/prometheus/client_golang/prometheus"
)

type Option func(*cgroupCollector)

func WithCGroupFS(root fs.FS) Option {
	return func(c *cgroupCollector) {
		c.fs = root
	}
}

func WithCGroupGlob(pattern string) Option {
	return func(c *cgroupCollector) {
		if pattern == "" {
			c.glob = "*"
		} else {
			c.glob = pattern
		}
	}
}

func WithMemoryBaseCollectors() Option {
	return func(c *cgroupCollector) {
		c.singleCollectors["memory.min"] = collector{desc: prometheus.NewDesc("cgroup_memory_min_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.low"] = collector{desc: prometheus.NewDesc("cgroup_memory_low_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.high"] = collector{desc: prometheus.NewDesc("cgroup_memory_high_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.max"] = collector{desc: prometheus.NewDesc("cgroup_memory_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.current"] = collector{desc: prometheus.NewDesc("cgroup_memory_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
	}
}

func WithMemorySwapCollectors() Option {
	return func(c *cgroupCollector) {
		c.singleCollectors["memory.swap.high"] = collector{desc: prometheus.NewDesc("cgroup_memory_swap_high_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.swap.max"] = collector{desc: prometheus.NewDesc("cgroup_memory_swap_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.swap.current"] = collector{desc: prometheus.NewDesc("cgroup_memory_swap_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
	}
}

func WithMemoryZSwapCollectors() Option {
	return func(c *cgroupCollector) {
		c.singleCollectors["memory.zswap.max"] = collector{desc: prometheus.NewDesc("cgroup_memory_zswap_max_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["memory.zswap.current"] = collector{desc: prometheus.NewDesc("cgroup_memory_zswap_current_bytes", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
	}
}

func WithMemoryNUMAStatCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"anon":                     {desc: prometheus.NewDesc("cgroup_memory_numa_anon_bytes", "Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS)", []string{"numa_node", "cgroup"}, nil)},
			"file":                     {desc: prometheus.NewDesc("cgroup_memory_numa_file_bytes", "Amount of memory used to cache filesystem data, including tmpfs and shared memory.", []string{"numa_node", "cgroup"}, nil)},
			"kernel_stack":             {desc: prometheus.NewDesc("cgroup_memory_numa_kernel_stack_bytes", "Amount of memory allocated to kernel stacks.", []string{"numa_node", "cgroup"}, nil)},
			"pagetables":               {desc: prometheus.NewDesc("cgroup_memory_numa_pagetables_bytes", "Amount of memory allocated for page tables.", []string{"numa_node", "cgroup"}, nil)},
			"sec_pagetables":           {desc: prometheus.NewDesc("cgroup_memory_numa_sec_pagetables_bytes", "Amount of memory allocated for secondary page tables.", []string{"numa_node", "cgroup"}, nil)},
			"shmem":                    {desc: prometheus.NewDesc("cgroup_memory_numa_shmem_bytes", "Amount of cached filesystem data that is swap-backed, such as tmpfs, shm segments, shared anonymous mmap()s", []string{"numa_node", "cgroup"}, nil)},
			"file_mapped":              {desc: prometheus.NewDesc("cgroup_memory_numa_file_mapped_bytes", "Amount of cached filesystem data mapped with mmap()", []string{"numa_node", "cgroup"}, nil)},
			"file_dirty":               {desc: prometheus.NewDesc("cgroup_memory_numa_file_dirty_bytes", "Amount of cached filesystem data that was modified but not yet written back to disk", []string{"numa_node", "cgroup"}, nil)},
			"file_writeback":           {desc: prometheus.NewDesc("cgroup_memory_numa_file_writeback_bytes", "Amount of cached filesystem data that was modified and is currently being written back to disk", []string{"numa_node", "cgroup"}, nil)},
			"swapcached":               {desc: prometheus.NewDesc("cgroup_memory_numa_swapcached_bytes", "Amount of swap cached in memory.", []string{"numa_node", "cgroup"}, nil)},
			"anon_thp":                 {desc: prometheus.NewDesc("cgroup_memory_numa_anon_thp_bytes", "Amount of memory used in anonymous mappings backed by transparent hugepages", []string{"numa_node", "cgroup"}, nil)},
			"file_thp":                 {desc: prometheus.NewDesc("cgroup_memory_numa_file_thp_bytes", "Amount of cached filesystem data backed by transparent hugepages", []string{"numa_node", "cgroup"}, nil)},
			"shmem_thp":                {desc: prometheus.NewDesc("cgroup_memory_numa_shmem_thp_bytes", "Amount of shm, tmpfs, shared anonymous mmap()s backed by transparent hugepages", []string{"numa_node", "cgroup"}, nil)},
			"inactive_anon":            {desc: prometheus.NewDesc("cgroup_memory_numa_inactive_anon_bytes", "Amount of memory on the inactive anonymous list", []string{"numa_node", "cgroup"}, nil)},
			"active_anon":              {desc: prometheus.NewDesc("cgroup_memory_numa_active_anon_bytes", "Amount of memory on the active anonymous list", []string{"numa_node", "cgroup"}, nil)},
			"inactive_file":            {desc: prometheus.NewDesc("cgroup_memory_numa_inactive_file_bytes", "Amount of memory on the inactive file list", []string{"numa_node", "cgroup"}, nil)},
			"active_file":              {desc: prometheus.NewDesc("cgroup_memory_numa_active_file_bytes", "Amount of memory on the active file list", []string{"numa_node", "cgroup"}, nil)},
			"unevictable":              {desc: prometheus.NewDesc("cgroup_memory_numa_unevictable_bytes", "Amount of memory that cannot be reclaimed", []string{"numa_node", "cgroup"}, nil)},
			"slab_reclaimable":         {desc: prometheus.NewDesc("cgroup_memory_numa_slab_reclaimable_bytes", "Amount of slab memory that might be reclaimed.", []string{"numa_node", "cgroup"}, nil)},
			"slab_unreclaimable":       {desc: prometheus.NewDesc("cgroup_memory_numa_slab_unreclaimable_bytes", "Amount of slab memory that cannot be reclaimed under memory pressure.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_refault_anon":  {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_refault_anon", "Number of refaults of previously evicted anonymous pages.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_refault_file":  {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_refault_file", "Number of refaults of previously evicted file pages.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_activate_anon": {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_activate_anon", "Number of refaulted anonymous pages that were immediately activated.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_activate_file": {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_activate_file", "Number of refaulted file pages that were immediately activated.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_restore_anon":  {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_restore_anon", "Number of restored anonymous pages detected as an active workingset before they got reclaimed.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_restore_file":  {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_restore_file", "Number of restored file pages detected as an active workingset before they got reclaimed.", []string{"numa_node", "cgroup"}, nil)},
			"workingset_nodereclaim":   {desc: prometheus.NewDesc("cgroup_memory_numa_workingset_nodereclaim", "Number of times a shadow node has been reclaimed.", []string{"numa_node", "cgroup"}, nil)},
		}

		c.multipleCollectors["memory.numa_stat"] = multipleCollector{descs: descs, collect: collectNumaStat}
	}
}

func WithMemoryStatCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
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
		}

		c.multipleCollectors["memory.stat"] = multipleCollector{descs: descs, collect: collectFlatKeyed(prometheus.GaugeValue)}
	}
}

func WithMemoryEventsCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"low":            {desc: prometheus.NewDesc("cgroup_memory_events_low_total", "", []string{"cgroup"}, nil)},
			"high":           {desc: prometheus.NewDesc("cgroup_memory_events_high_total", "", []string{"cgroup"}, nil)},
			"max":            {desc: prometheus.NewDesc("cgroup_memory_events_max_total", "", []string{"cgroup"}, nil)},
			"oom":            {desc: prometheus.NewDesc("cgroup_memory_events_oom_total", "", []string{"cgroup"}, nil)},
			"oom_kill":       {desc: prometheus.NewDesc("cgroup_memory_events_oom_kill_total", "", []string{"cgroup"}, nil)},
			"oom_group_kill": {desc: prometheus.NewDesc("cgroup_memory_events_oom_group_kill_total", "", []string{"cgroup"}, nil)},
		}

		c.multipleCollectors["memory.events"] = multipleCollector{descs: descs, collect: collectFlatKeyed(prometheus.CounterValue)}
	}
}

func WithMemoryPressureCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"some": {desc: prometheus.NewDesc("cgroup_memory_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"full": {desc: prometheus.NewDesc("cgroup_memory_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
		}

		c.multipleCollectors["memory.pressure"] = multipleCollector{descs: descs, collect: collectPressure}
	}
}

func WithCPUStatCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"usage_usec":                 {desc: prometheus.NewDesc("cgroup_cpu_usage_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"user_usec":                  {desc: prometheus.NewDesc("cgroup_cpu_user_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"system_usec":                {desc: prometheus.NewDesc("cgroup_cpu_system_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"nr_periods":                 {desc: prometheus.NewDesc("cgroup_cpu_periods_total", "", []string{"cgroup"}, nil)},
			"nr_throttled":               {desc: prometheus.NewDesc("cgroup_cpu_throttled_total", "", []string{"cgroup"}, nil)},
			"throttled_usec":             {desc: prometheus.NewDesc("cgroup_cpu_throttled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"nr_bursts":                  {desc: prometheus.NewDesc("cgroup_cpu_bursts_total", "", []string{"cgroup"}, nil)},
			"burst_usec":                 {desc: prometheus.NewDesc("cgroup_cpu_burst_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"core_sched.force_idle_usec": {desc: prometheus.NewDesc("cgroup_cpu_core_sched_force_idle_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
		}

		c.multipleCollectors["cpu.stat"] = multipleCollector{descs: descs, collect: collectFlatKeyed(prometheus.CounterValue)}
	}
}

func WithCPUPressureCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"some": {desc: prometheus.NewDesc("cgroup_cpu_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"full": {desc: prometheus.NewDesc("cgroup_cpu_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
		}

		c.multipleCollectors["cpu.pressure"] = multipleCollector{descs: descs, collect: collectPressure}
	}
}

func WithIOStatCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"rbytes": {desc: prometheus.NewDesc("cgroup_io_read_bytes_total", "", []string{"device", "cgroup"}, nil)},
			"wbytes": {desc: prometheus.NewDesc("cgroup_io_write_bytes_total", "", []string{"device", "cgroup"}, nil)},
			"dbytes": {desc: prometheus.NewDesc("cgroup_io_discard_bytes_total", "", []string{"device", "cgroup"}, nil)},
			"rios":   {desc: prometheus.NewDesc("cgroup_io_read_operations_total", "", []string{"device", "cgroup"}, nil)},
			"wios":   {desc: prometheus.NewDesc("cgroup_io_write_operations_total", "", []string{"device", "cgroup"}, nil)},
			"dios":   {desc: prometheus.NewDesc("cgroup_io_discard_operations_total", "", []string{"device", "cgroup"}, nil)},
		}

		c.multipleCollectors["io.stat"] = multipleCollector{descs: descs, collect: collectIOStat}
	}
}

func WithIOPressureCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"some": {desc: prometheus.NewDesc("cgroup_io_pressure_waiting_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
			"full": {desc: prometheus.NewDesc("cgroup_io_pressure_stalled_seconds_total", "", []string{"cgroup"}, nil), modifier: microSecondsToSeconds},
		}

		c.multipleCollectors["cpu.pressure"] = multipleCollector{descs: descs, collect: collectPressure}
	}
}

func WithPIDBaseCollectors() Option {
	return func(c *cgroupCollector) {
		c.singleCollectors["pids.current"] = collector{desc: prometheus.NewDesc("cgroup_pids_current", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
		c.singleCollectors["pids.max"] = collector{desc: prometheus.NewDesc("cgroup_pids_max", "", []string{"cgroup"}, nil), collect: collectSingleValue(prometheus.GaugeValue)}
	}
}

func WithPIDEventsCollectors() Option {
	return func(c *cgroupCollector) {
		descs := map[string]desc{
			"max": {desc: prometheus.NewDesc("cgroup_pids_events_max_total", "", []string{"cgroup"}, nil)},
		}

		c.multipleCollectors["pid.events"] = multipleCollector{descs: descs, collect: collectFlatKeyed(prometheus.CounterValue)}
	}
}

var CollectorStatOptions = map[string]Option{
	"memory_base":      WithMemoryBaseCollectors(),
	"memory_swap":      WithMemorySwapCollectors(),
	"memory_zswap":     WithMemoryZSwapCollectors(),
	"memory_numa_stat": WithMemoryNUMAStatCollectors(),
	"memory_stat":      WithMemoryStatCollectors(),
	"memory_events":    WithMemoryEventsCollectors(),
	"memory_pressure":  WithMemoryPressureCollectors(),
	"cpu_stat":         WithCPUStatCollectors(),
	"cpu_pressure":     WithCPUPressureCollectors(),
	"io_stat":          WithIOStatCollectors(),
	"io_pressure":      WithIOPressureCollectors(),
	"pid_base":         WithPIDBaseCollectors(),
	"pid_events":       WithPIDEventsCollectors(),
}
