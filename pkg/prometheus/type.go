package prometheus

type MetricType struct {
	NodeName string `json:"node_name"`
}

type ResultType struct {
	Metric MetricType    `json:"metric"`
	Value  []interface{} `json:"value"`
}

type QueryData struct {
	ResultType string       `json:"resultType"`
	Result     []ResultType `json:"result"`
}

type QueryInfo struct {
	Status string    `json:"status"`
	Data   QueryData `json:"data"`
}
