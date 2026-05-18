package recovery

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	healthmodel "health-monitor/pkg/models"
	healthstate "health-monitor/pkg/state"
)

// HealthMonitorStateChecker reads recovery recheck controls from the
// health-monitor realtime StateManager cache.
type HealthMonitorStateChecker struct {
	sm *healthstate.StateManager
}

func NewHealthMonitorStateChecker(sm *healthstate.StateManager) *HealthMonitorStateChecker {
	return &HealthMonitorStateChecker{sm: sm}
}

func NewRecoveryServiceWithHealthMonitorState(registry *PlanRegistry, client ContainerClient, healthSM *healthstate.StateManager, sm StateManager, cfg RecoveryServiceConfig) *RecoveryService {
	return NewRecoveryService(registry, client, NewHealthMonitorStateChecker(healthSM), sm, cfg)
}

func (c *HealthMonitorStateChecker) CheckBoolean(ctx context.Context, metricName string) (bool, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return false, false, ctx.Err()
	default:
	}

	if c == nil || c.sm == nil {
		return false, false, fmt.Errorf("health-monitor state manager is nil")
	}

	metricName = strings.TrimSpace(metricName)
	if metricName == "" {
		return false, false, fmt.Errorf("metric name is empty")
	}

	for _, metric := range c.candidateMetrics(metricName) {
		if metric == nil {
			continue
		}
		actual, exists, err := boolFromHealthMetric(metric.GetData(), metricName)
		if err != nil {
			return false, true, err
		}
		if exists {
			return actual, true, nil
		}
	}

	return false, false, nil
}

func (c *HealthMonitorStateChecker) candidateMetrics(metricName string) []healthstate.Metric {
	seen := make(map[string]bool)
	var metrics []healthstate.Metric

	addByID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		if metric, ok := c.sm.GetLatestState(id); ok {
			metrics = append(metrics, metric)
			seen[id] = true
		}
	}

	for _, id := range metricStateHints(metricName) {
		addByID(id)
	}
	for _, metric := range c.sm.GetAllLatestStates() {
		if metric == nil {
			continue
		}
		id := metric.GetID()
		if seen[id] {
			continue
		}
		metrics = append(metrics, metric)
		seen[id] = true
	}

	return metrics
}

func metricStateHints(metricName string) []string {
	switch metricName {
	case "Communication_power_status",
		"GNSSA_power_status",
		"GNSSB_power_status",
		"Gyroscope_power_status",
		"MEMS_power_status",
		"StarTrackerl_power_status",
		"MomentumWheel_power_status":
		return []string{"power"}
	default:
		return []string{
			"power",
			"thermal",
			"comm",
			"attitude_orbit_control",
			"momentum_wheel",
		}
	}
}

func boolFromHealthMetric(data interface{}, metricName string) (bool, bool, error) {
	value, exists := lookupHealthMetricValue(data, metricName)
	if !exists {
		return false, false, nil
	}
	actual, ok := boolValueFromInterface(value)
	if !ok {
		return false, true, fmt.Errorf("state metric %s is not boolean-compatible: %T", metricName, value)
	}
	return actual, true, nil
}

func lookupHealthMetricValue(data interface{}, metricName string) (interface{}, bool) {
	switch v := data.(type) {
	case *healthmodel.BusinessMetrics:
		if v == nil {
			return nil, false
		}
		return lookupHealthMetricValue(v.Data, metricName)
	case healthmodel.BusinessMetrics:
		return lookupHealthMetricValue(v.Data, metricName)
	case *healthmodel.AttitudeOrbitControlMetrics:
		if v == nil {
			return nil, false
		}
		return lookupMapValue(v.Values, metricName)
	case healthmodel.AttitudeOrbitControlMetrics:
		return lookupMapValue(v.Values, metricName)
	case *healthmodel.RunMgrMetrics:
		if v == nil {
			return nil, false
		}
		if val, ok := lookupMapValue(v.Payload, metricName); ok {
			return val, true
		}
	case healthmodel.RunMgrMetrics:
		if val, ok := lookupMapValue(v.Payload, metricName); ok {
			return val, true
		}
	}

	return lookupReflectValue(reflect.ValueOf(data), metricName)
}

func lookupReflectValue(v reflect.Value, metricName string) (interface{}, bool) {
	if !v.IsValid() {
		return nil, false
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		return lookupReflectMapValue(v, metricName)
	case reflect.Struct:
		return lookupStructValue(v, metricName)
	default:
		return nil, false
	}
}

func lookupStructValue(v reflect.Value, metricName string) (interface{}, bool) {
	candidates := fieldNameCandidates(metricName)
	t := v.Type()
	for _, name := range candidates {
		if field, ok := t.FieldByName(name); ok && field.PkgPath == "" {
			fv := v.FieldByIndex(field.Index)
			if fv.CanInterface() {
				return fv.Interface(), true
			}
		}
	}

	normalizedMetricName := normalizeLookupKey(metricName)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fv := v.Field(i)
		if field.Name == "Values" || field.Name == "Payload" {
			if val, ok := lookupReflectValue(fv, metricName); ok {
				return val, true
			}
		}
		if normalizeLookupKey(field.Name) == normalizedMetricName && fv.CanInterface() {
			return fv.Interface(), true
		}
	}

	return nil, false
}

func lookupMapValue(m map[string]interface{}, metricName string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	if val, ok := m[metricName]; ok {
		return val, true
	}
	normalizedMetricName := normalizeLookupKey(metricName)
	for key, val := range m {
		if normalizeLookupKey(key) == normalizedMetricName {
			return val, true
		}
	}
	return nil, false
}

func lookupReflectMapValue(v reflect.Value, metricName string) (interface{}, bool) {
	if v.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	key := reflect.ValueOf(metricName)
	if key.Type().AssignableTo(v.Type().Key()) {
		if val := v.MapIndex(key); val.IsValid() && val.CanInterface() {
			return val.Interface(), true
		}
	}

	normalizedMetricName := normalizeLookupKey(metricName)
	iter := v.MapRange()
	for iter.Next() {
		mapKey := iter.Key()
		if normalizeLookupKey(mapKey.String()) == normalizedMetricName {
			val := iter.Value()
			if val.CanInterface() {
				return val.Interface(), true
			}
		}
	}
	return nil, false
}

func fieldNameCandidates(metricName string) []string {
	snake := snakeToExported(metricName)
	return []string{
		metricName,
		snake,
		strings.TrimSuffix(snake, "Status") + "Status",
	}
}

func snakeToExported(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func normalizeLookupKey(s string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(s)))
}

func boolValueFromInterface(v interface{}) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		return parseActualBoolString(x)
	case []byte:
		return parseActualBoolString(string(x))
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return false, false
	}
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return false, false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() != 0, true
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0, true
	default:
		return false, false
	}
}

func parseActualBoolString(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "1", "open", "opened", "enable", "enabled":
		return true, true
	case "off", "false", "0", "close", "closed", "disable", "disabled":
		return false, true
	default:
		n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err == nil {
			return n != 0, true
		}
		return false, false
	}
}
