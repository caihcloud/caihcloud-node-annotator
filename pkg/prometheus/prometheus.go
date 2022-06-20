package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/caihcloud/node-annotator/conf"
	"k8s.io/klog"
)

func GetPromResult(url string, result interface{}) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(result)
	if err != nil {
		klog.Errorln(debug.Stack())
		debug.PrintStack()
		return err
	}
	return nil
}

// query metric by prom api
func QueryMetric(endpoint string, query string) (*QueryInfo, error) {
	info := &QueryInfo{}
	ustr := endpoint + "/api/v1/query?query=" + query
	u, err := url.Parse(ustr)
	if err != nil {
		return info, err
	}
	u.RawQuery = u.Query().Encode()

	err = GetPromResult(u.String(), &info)
	if err != nil {
		klog.Errorln(info)
		return info, err
	}
	return info, nil
}

func ParseMetricTimeAndValueFloat64(query string) (int64, float64, error) {
	info, err := QueryMetric(conf.PrometheusUrl, query)
	if err != nil {
		return 0, 0, err
	}
	data := &info.Data
	if len(data.Result) <= 0 {
		return 0, 0, fmt.Errorf("get metric empty")
	}
	recoreTimestamp := int64(data.Result[0].Value[0].(float64))
	value, err := strconv.ParseFloat(data.Result[0].Value[1].(string), 64)
	if err != nil {
		return 0, 0, err
	}
	return recoreTimestamp, value, nil
}
