package annotator

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/robfig/cron"

	"github.com/caihcloud/node-annotator/conf"
	"github.com/caihcloud/node-annotator/pkg/client"
	"github.com/caihcloud/node-annotator/pkg/prometheus"

	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

var runJobSpecs []ResourceSpec

// patch node annotation
func UpdateNodeAnnotation(nodeName string, annotationKey string, annotationNewValue string) error {
	node, err := client.K8sClientSet.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	annotations := node.Annotations
	if len(node.Annotations) == 0 {
		annotations = make(map[string]string)
	}
	annotations[annotationKey] = annotationNewValue

	patchData := map[string]interface{}{"metadata": map[string]map[string]string{"annotations": annotations}}
	playLoadBytes, _ := json.Marshal(patchData)

	_, err = client.K8sClientSet.CoreV1().Nodes().Patch(nodeName, types.StrategicMergePatchType, playLoadBytes)
	if err != nil {
		return err
	}
	klog.Infof("annotation node [%s]: %s=%s", nodeName, annotationKey, annotationNewValue)
	return nil
}

// annotation node with metrics
func AnnotaClusterNodes(spec ResourceSpec, interval string, data prometheus.QueryInfo) {
	if len(data.Data.Result) <= 0 {
		klog.Errorf("get metric [%s] error", spec.Name)
		return
	}
	for _, r := range data.Data.Result {
		value, err := strconv.ParseFloat(r.Value[1].(string), 64)
		if err != nil {
			klog.Error(err.Error())
			continue
		}
		// [timestamp]:[interval]:[value]:[threshold]:[weight]
		anno := fmt.Sprintf("%d:%s:%.2f:%.2f:%.2f", int64(r.Value[0].(float64)), interval, value, spec.Threshold, spec.Weight)
		if err := UpdateNodeAnnotation(r.Metric.NodeName, conf.LableKeyPrefix+spec.Name, anno); err != nil {
			klog.Error(err)
			continue
		}
	}
}

func runJob(specs []ResourceSpec, interval string) {
	for _, metric := range specs {
		info, err := prometheus.QueryMetric(conf.PrometheusUrl, metric.Expr)
		if err != nil {
			klog.Error("QueryMetric error: " + err.Error())
		} else {
			AnnotaClusterNodes(metric, interval, *info)
		}
	}
}

// run cron job
func Run() {
	klog.Infoln("node annotator start...")
	c := cron.New()
	c.AddFunc("0 */5 * * * *", func() {
		prometheus.ReflushPushGateway()
		runJob(runJobSpecs, "300")
	})

	c.Start()
	defer c.Stop()
	select {}
}

func PreRun() {
	klog.Infoln("parseConfig...")
	parseConfig()
	runJob(runJobSpecs, "300")
}

func parseConfig() {
	c := &Config{}
	viper.SetConfigType("yaml")
	viper.SetConfigFile(conf.ConfigFile)
	err := viper.ReadInConfig()
	if err != nil {
		klog.Fatal("read config is failed err:", err)
	}

	err = viper.Unmarshal(c)
	if err != nil {
		klog.Fatal("unmarshal config is failed, err:", err)
	}

	runJobSpecs = c.Metrics

	if len(runJobSpecs) == 0 {
		klog.Fatal("get empty args")
	}
	for _, metric := range runJobSpecs {
		if metric.Name == "" || metric.Expr == "" {
			klog.Fatal("args metric has no name or expr")
		}
	}
}
