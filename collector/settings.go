package collector

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type SettingsCollector struct {
	client *Client
	logger *logrus.Logger

	datePattern string

	fieldsLimit      *prometheus.Desc
	fieldsGroupLimit *prometheus.Desc
}

func NewSettingsCollector(logger *logrus.Logger, client *Client, labels, labels_group []string, datepattern string,
	constLabels prometheus.Labels) *SettingsCollector {

	return &SettingsCollector{
		client:      client,
		logger:      logger,
		datePattern: datepattern,
		fieldsLimit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "fields_limit", "total"),
			"Total limit of fields of each index to date", labels, constLabels,
		),
		fieldsGroupLimit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "fields_group_limit", "total"),
			"Total limit of fields of each index group to date", labels_group, constLabels,
		),
	}
}

func (c *SettingsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fieldsLimit
	ch <- c.fieldsGroupLimit
}

func (c *SettingsCollector) Collect(ch chan<- prometheus.Metric) {
	today := todayFunc(c.datePattern)
	indicesPattern := indicesPatternFunc(today)

	settings, err := c.client.GetSettings([]string{indicesPattern})
	if err != nil {
		c.logger.Fatalf("error getting indices settings: %v", err)
	}

	fieldsGroupLimit := make(map[string]float64)
	for index, v := range settings {
		// Create variable with index prefix
		indexGrouplabel := indexGroupLabelFunc(index, today)

		data, ok := v.(map[string]interface{})
		if !ok {
			c.logger.Error("got invalid index setttings for: %s", index)
			continue
		}

		path := "index.mapping.total_fields.limit"
		limit, ok := walk(data, "settings."+path)
		if !ok {
			limit, ok = walk(data, "defaults."+path)
			if !ok {
				c.logger.Errorf("%q was not found for: %s", path, index)
				continue
			}
		}

		if s, ok := limit.(string); ok {
			if v, err := strconv.ParseFloat(s, 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.fieldsLimit, prometheus.GaugeValue, v, index, indexGrouplabel)
				fieldsGroupLimit[indexGrouplabel] += v
			} else {
				c.logger.Errorf("error parsing %q value for: %s: %v ", path, index, err)
			}
		} else {
			c.logger.Errorf("got invalid %q value for: %s value: %#v", path, index, limit)
		}
	}

	for indexGroup, v := range fieldsGroupLimit {
		ch <- prometheus.MustNewConstMetric(c.fieldsGroupLimit, prometheus.GaugeValue, v, indexGroup)
	}
}
