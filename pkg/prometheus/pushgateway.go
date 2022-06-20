package prometheus

import (
	"fmt"
	"reflect"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/caihcloud/node-annotator/conf"
	"github.com/caihcloud/node-annotator/pkg/client"
)

var Counter = map[string]prometheus.Counter{
	conf.TotalScheduleCount: prometheus.NewCounter(prometheus.CounterOpts{
		Name:        conf.TotalScheduleCount,
		Help:        conf.TotalScheduleCount,
		ConstLabels: prometheus.Labels{"type": conf.TotalScheduleCount},
	}),
	conf.EffectiveDynamicScheduleCount: prometheus.NewCounter(prometheus.CounterOpts{
		Name:        conf.EffectiveDynamicScheduleCount,
		Help:        conf.EffectiveDynamicScheduleCount,
		ConstLabels: prometheus.Labels{"type": conf.EffectiveDynamicScheduleCount},
	}),
}

var registry = prometheus.NewRegistry()

func InitPushGatewayCounter() {
	klog.Infoln("init pushgateway counter...")
	registry.MustRegister(Counter[conf.TotalScheduleCount], Counter[conf.EffectiveDynamicScheduleCount])
	Counter[conf.TotalScheduleCount].Add(0)
	Counter[conf.EffectiveDynamicScheduleCount].Add(0)
	if err := push.New(conf.PushGatewayUrl, "schedulerStatus").Gatherer(registry).Push(); err != nil {
		klog.Fatal(err)
	}
}

func PushGatewayIncCounter(name string) error {
	Counter[name].Inc()
	if err := push.New(conf.PushGatewayUrl, "schedulerStatus").Gatherer(registry).Push(); err != nil {
		klog.Errorln("pushgateway Inc counter error:", err)
		return err
	}
	klog.Infoln("pushgateway Inc counter done: ", name)
	return nil
}

/**
	监听Pod调度事件
**/
func InformerPodScheduler() {
	klog.Infoln("start InformerPodScheduler, watch scheduler name: ", conf.DynamicSchedulerName)

	watchlist := cache.NewListWatchFromClient(client.K8sClientSet.CoreV1().RESTClient(), "pods", k8smetav1.NamespaceAll, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&k8sv1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldPodObj, oldOk := oldObj.(*k8sv1.Pod)
				newPodObj, newOk := newObj.(*k8sv1.Pod)
				if oldOk && newOk {
					if err := PodChange(oldPodObj, newPodObj); err != nil {
						klog.Errorln(err.Error())
					}
				} else {
					klog.Errorln("obj is not Pod")
					klog.Errorln(reflect.TypeOf(oldObj))
					klog.Errorln(reflect.TypeOf(newObj))
				}
			},
		},
	)
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	for {
		time.Sleep(time.Second * 5)
	}
}

// fixbug: interval reflush
func ReflushPushGateway() {
	if conf.PushGatewayUrl == "" {
		return
	}
	Counter[conf.TotalScheduleCount].Add(0)
	Counter[conf.EffectiveDynamicScheduleCount].Add(0)
	if err := push.New(conf.PushGatewayUrl, "schedulerStatus").Gatherer(registry).Push(); err != nil {
		klog.Errorln(err)
	}
}

/**
	监听Pod更新事件，如果:
	1. pod的调度器名指定动态调度器
	2. pod更新node_name
	3. pod的调度时间大于上次记录的时间
	4. pod的调度时间与当前时间相比不超过1min（不记录太老的事件）

	获取当前调度节点的cpu利用率：scheduler_cpu_usage_active_percent
	获取当前调度节点的mem利用率：scheduler_mem_usage_active_percent
	则调度次数scheduler_total_schedule_count +1
	计算是否有效调度，如果是则有效调度scheduler_effective_dynamic_schedule_count +1
**/
func PodChange(oldObj *k8sv1.Pod, newObj *k8sv1.Pod) error {
	//1. 判断是否指定动态调度器
	if newObj.Spec.SchedulerName != conf.DynamicSchedulerName {
		return nil
	}

	//2. 判断是否调度成功事件（更新node_name）
	if newObj.Spec.NodeName == "" || (oldObj.Spec.NodeName != "" && newObj.Spec.NodeName != "") {
		return nil
	}

	klog.Infof("[%s] pod %s scheduler to node %s...", conf.DynamicSchedulerName, newObj.Name, newObj.Spec.NodeName)
	//3. 判断pod的调度时间与当前时间相比
	scheduledTimestamp := time.Now()
	nowTimestamp := scheduledTimestamp
	for _, c := range newObj.Status.Conditions {
		if c.Type == "PodScheduled" {
			scheduledTimestamp = c.LastTransitionTime.Time
			if nowTimestamp.Unix()-scheduledTimestamp.Unix() > 60 {
				err := fmt.Errorf("[IGNORE]: pod is too old to recore [%s : %s]", nowTimestamp.Format("2006-01-02 15:04:05"), scheduledTimestamp.Format("2006-01-02 15:04:05"))
				return err
			}
		}
	}
	if scheduledTimestamp == nowTimestamp {
		err := fmt.Errorf("[IGNORE]: get PodScheduled time error")
		return err
	}

	//4. 获取记录时间与pod调度时间相比，如果记录时间大于pod调度时间，则跳过
	lastTimestamp, _, err := ParseMetricTimeAndValueFloat64(conf.TotalScheduleCount)
	if err != nil {
		klog.Errorln("recored may empty: ", err.Error())
	} else {
		if lastTimestamp-scheduledTimestamp.Unix() > 60 {
			err := fmt.Errorf("[IGNORE]: pod is too old to recore <%s : %s>", time.Unix(lastTimestamp, 0).Format("2006-01-02 15:04:05"), scheduledTimestamp.Format("2006-01-02 15:04:05"))
			return err
		}
	}

	//5. 计算所有节点的cpu平均利用率和调度到的节点的cpu利用率
	_, nodeCPUUsage, err := ParseMetricTimeAndValueFloat64(fmt.Sprintf("%s{node_name='%s'}", conf.NodeCPUUsage, newObj.Spec.NodeName))
	if err != nil {
		klog.Errorf("%s{node_name=%s}\n", conf.NodeCPUUsage, newObj.Spec.NodeName)
		return err
	}

	_, cpuUtilizationTotalAvg, err := ParseMetricTimeAndValueFloat64(conf.CPUUtilizationTotalAvg)
	if err != nil {
		klog.Errorln(conf.CPUUtilizationTotalAvg)
		return err
	}

	klog.Infof("%s[%s]: %f", conf.NodeCPUUsage, newObj.Spec.NodeName, nodeCPUUsage)
	klog.Infof("%s: %f", conf.CPUUtilizationTotalAvg, cpuUtilizationTotalAvg)

	//6. 总调度次数+1
	err = PushGatewayIncCounter(conf.TotalScheduleCount)
	if err != nil {
		klog.Errorln(conf.TotalScheduleCount)
		return err
	}

	//7. 判断是否有效调度,如果是则有效调度次数+1
	if nodeCPUUsage <= cpuUtilizationTotalAvg {
		err := PushGatewayIncCounter(conf.EffectiveDynamicScheduleCount)
		if err != nil {
			klog.Errorln(conf.EffectiveDynamicScheduleCount)
			return err
		}
	}

	return nil
}
