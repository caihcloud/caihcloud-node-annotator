package conf

// note: default cofig for deug
const (
	LableKeyPrefix = "caih.com/"
)

var (
	KubeconfigPath       = ""
	PrometheusUrl        = ""
	PushGatewayUrl       = ""
	DynamicSchedulerName = ""
	LeaseLockNamespace   = ""
	LeaseLockName        = ""
	ConfigFile           = ""
)

const (
	NodeCPUUsage                  = "scheduler_cpu_usage_active_percent"
	NodeMemUsage                  = "scheduler_mem_usage_active_percent"
	CPUUtilizationTotalAvg        = "scheduler_cpu_utilization_total_avg_percent"
	MemUtilizationTotalAvg        = "scheduler_mem_utilization_total_avg_percent"
	TotalScheduleCount            = "scheduler_total_schedule_count"
	EffectiveDynamicScheduleCount = "scheduler_effective_dynamic_schedule_count"
	EffectiveScheduleRatio        = "scheduler_effective_schedule_ratio"
)

const SchedulerPluginName = "caihcloud-real-node-load-plugin"
