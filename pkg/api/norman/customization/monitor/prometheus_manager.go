package monitor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promapiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	legendFormat = regexp.MustCompile(`\[\[\s*(.+?)\s*\]\]`)
)

type Queries struct {
	ctx context.Context
	api promapiv1.API
	eg  *errgroup.Group
}

func NewPrometheusQuery(ctx context.Context, clusterName, authToken, svcNamespace, svcName, svcPort string, dialerFactory dialer.Factory, userContext *config.UserContext) (*Queries, error) {
	ep, err := userContext.Core.Endpoints(svcNamespace).Get(svcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s/%s endpoints, %v", svcNamespace, svcName, err)
	}

	ip := pickEndpointAddress(ep)
	if ip == "" {
		return nil, fmt.Errorf("failed to pick endpoint address")
	}

	dial, err := dialerFactory.ClusterDialer(clusterName)
	if err != nil {
		return nil, fmt.Errorf("get dail from usercontext failed, %v", err)
	}

	endpoint := fmt.Sprintf("http://%s:%s", ip, svcPort)

	api, err := newPrometheusAPI(dial, endpoint, authToken)
	if err != nil {
		return nil, err
	}
	return newQuery(ctx, api), nil
}

func (q *Queries) QueryRange(query *PrometheusQuery) ([]*TimeSeries, error) {
	value, err := q.api.QueryRange(q.ctx, query.Expr, query.getRange())
	if err != nil {
		return nil, fmt.Errorf("query range failed, %v, expression: %s", err, query.Expr)
	}
	seriesSlice, err := parseMatrix(value, query)
	if err != nil {
		return nil, fmt.Errorf("parse prometheus query result failed, %v", err)
	}
	return seriesSlice, nil
}

func (q *Queries) Query(query *PrometheusQuery) ([]*TimeSeries, error) {

	value, err := q.api.Query(q.ctx, query.Expr, time.Now())
	if err != nil {
		return nil, fmt.Errorf("query range failed, %v, expression: %s", err, query.Expr)
	}
	series, err := parseVector(value, query)
	if err != nil {
		return nil, fmt.Errorf("parse prometheus query result failed, %v", err)
	}

	if series == nil {
		return nil, nil
	}

	return []*TimeSeries{series}, nil
}

func (q *Queries) Do(querys []*PrometheusQuery) (map[string][]*TimeSeries, error) {
	smap := &sync.Map{}
	for _, v := range querys {
		query := v
		q.eg.Go(func() error {
			var seriesSlice []*TimeSeries
			var err error
			if query.IsInstanceQuery {
				seriesSlice, err = q.Query(query)
			} else {
				seriesSlice, err = q.QueryRange(query)
			}
			if err != nil {
				return err
			}

			if seriesSlice != nil {
				smap.Store(query.ID, seriesSlice)
			}
			return nil
		})
	}
	if err := q.eg.Wait(); err != nil {
		return nil, err
	}

	rtn := make(map[string][]*TimeSeries)
	smap.Range(func(k, v interface{}) bool {
		key1, key2, _ := parseID(k.(string))
		key := fmt.Sprintf("%s_%s", key1, key2)

		series := v.([]*TimeSeries)
		if len(series) != 0 {
			rtn[key] = append(rtn[key], series...)
		}
		return true
	})

	return rtn, nil
}

func (q *Queries) GetLabelValues(labelName string) ([]string, error) {
	value, err := q.api.LabelValues(q.ctx, labelName)
	if err != nil {
		return nil, fmt.Errorf("get prometheus metric list failed, %v", err)
	}

	var metricNames []string
	for _, v := range value {
		metricNames = append(metricNames, fmt.Sprint(v))
	}
	return metricNames, nil
}

func InitPromQuery(id string, start, end time.Time, step time.Duration, expr, format string, isInstanceQuery bool) *PrometheusQuery {
	return &PrometheusQuery{
		ID:              id,
		Start:           start,
		End:             end,
		Step:            step,
		Expr:            expr,
		LegendFormat:    format,
		IsInstanceQuery: isInstanceQuery,
	}
}

func newQuery(ctx context.Context, api promapiv1.API) *Queries {
	q := &Queries{
		ctx: ctx,
		api: api,
	}
	q.eg, q.ctx = errgroup.WithContext(q.ctx)
	return q
}

type authTransport struct {
	*http.Transport

	token string
}

func (auth authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.token))
	return auth.Transport.RoundTrip(req)
}

func newHTTPTransport(dial dialer.Dialer) *http.Transport {
	return &http.Transport{
		DialContext:           dial,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}
}

func newPrometheusAPI(dial dialer.Dialer, url, token string) (promapiv1.API, error) {
	auth := authTransport{
		Transport: newHTTPTransport(dial),
		token:     token,
	}

	cfg := promapi.Config{
		Address:      url,
		RoundTripper: auth,
	}

	client, err := promapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create prometheus client failed: %v", err)
	}
	return promapiv1.NewAPI(client), nil
}

func parseVector(value model.Value, query *PrometheusQuery) (*TimeSeries, error) {
	data, ok := value.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("Unsupported result format: %s", value.Type().String())
	}

	if data.Len() == 0 {
		return nil, nil
	}

	vec := data[0]
	series := TimeSeries{
		Name: formatLegend(vec.Metric, query),
		Tags: map[string]string{},
	}

	for k, v := range vec.Metric {
		series.Tags[string(k)] = string(v)
	}

	po, isValid := NewTimePoint(float64(vec.Value), float64(vec.Timestamp.Unix()*1000))
	if isValid {
		series.Points = append(series.Points, po)
		return &series, nil
	}

	return nil, nil
}

func parseMatrix(value model.Value, query *PrometheusQuery) ([]*TimeSeries, error) {
	data, ok := value.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("Unsupported result format: %s", value.Type().String())
	}

	if data.Len() == 0 {
		return nil, nil
	}

	var seriesSlice []*TimeSeries
	for _, v := range data {
		series := TimeSeries{
			Name: formatLegend(v.Metric, query),
			Tags: map[string]string{},
		}

		for k, v := range v.Metric {
			series.Tags[string(k)] = string(v)
		}

		for _, v := range v.Values {
			po, isValid := NewTimePoint(float64(v.Value), float64(v.Timestamp.Unix()*1000))
			if isValid {
				series.Points = append(series.Points, po)
			}
		}

		seriesSlice = append(seriesSlice, &series)
	}

	return seriesSlice, nil
}

func formatLegend(metric model.Metric, query *PrometheusQuery) string {
	if query.LegendFormat == "" {
		return metric.String()
	}

	result := legendFormat.ReplaceAllFunc([]byte(query.LegendFormat), func(in []byte) []byte {
		labelName := strings.Replace(string(in), "[[", "", 1)
		labelName = strings.Replace(labelName, "]]", "", 1)
		labelName = strings.TrimSpace(labelName)
		if val, exists := metric[model.LabelName(labelName)]; exists {
			return []byte(val)
		}

		return in
	})

	return string(result)
}

func pickEndpointAddress(ep *corev1.Endpoints) string {
	epSubsets := ep.Subsets
	if len(epSubsets) != 0 && len(epSubsets[0].Addresses) != 0 {
		return epSubsets[0].Addresses[0].IP
	}

	return ""
}
