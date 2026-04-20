package kubernetes

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"gopkg.in/yaml.v3"
)

const fluxKustomizationResourcePath = "/apis/kustomize.toolkit.fluxcd.io/v1"
const argoRolloutResourcePath = "/apis/argoproj.io/v1alpha1"
const kubernetesPodMetricsResourcePath = "/apis/metrics.k8s.io/v1beta1"

type ErrorSyncStatusReader struct {
	Err error
}

type ErrorNetworkExposureReader struct {
	Err error
}

type ErrorRolloutController struct {
	Err error
}

func (r ErrorSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}
	if r.Err == nil {
		return application.SyncInfo{}, nil
	}
	return application.SyncInfo{}, r.Err
}

func (r ErrorSyncStatusReader) ReadMany(ctx context.Context, records []application.Record) (map[string]application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Err == nil {
		return map[string]application.SyncInfo{}, nil
	}
	return nil, r.Err
}

func (r ErrorNetworkExposureReader) Read(ctx context.Context, record application.Record) (application.NetworkExposureInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.NetworkExposureInfo{}, err
	}

	observedAt := record.UpdatedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	if !record.LoadBalancerEnabled {
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusInternal,
			Message:     "현재는 내부 전용(ClusterIP) 서비스로 운영 중입니다.",
			ServiceType: "ClusterIP",
			ObservedAt:  observedAt,
		}, nil
	}

	message := "LoadBalancer 상태 조회에 실패했습니다."
	if r.Err != nil {
		message = fmt.Sprintf("LoadBalancer 상태 조회에 실패했습니다: %s", r.Err.Error())
	}

	return application.NetworkExposureInfo{
		Status:      application.NetworkExposureStatusError,
		Message:     message,
		ServiceType: "LoadBalancer",
		ObservedAt:  observedAt,
	}, nil
}

func (r ErrorRolloutController) GetRollout(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	if r.Err == nil {
		return application.RolloutInfo{}, nil
	}
	return application.RolloutInfo{}, r.Err
}

func (r ErrorRolloutController) Promote(ctx context.Context, record application.Record, full bool) (application.RolloutInfo, error) {
	return r.GetRollout(ctx, record)
}

func (r ErrorRolloutController) Abort(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	return r.GetRollout(ctx, record)
}

type FluxSyncStatusReader struct {
	Client                 *apiClient
	KustomizationNamespace string
}

type ServiceNetworkExposureReader struct {
	Client *apiClient
}

type PodMetricsReader struct {
	Client *apiClient
	Range  time.Duration
	Step   time.Duration
	Now    func() time.Time
}

type PodLogReader struct {
	Client *apiClient
	Now    func() time.Time
}

type ArgoRolloutController struct {
	Client *apiClient
}

func NewSyncStatusReader(cfg core.Config) application.StatusReader {
	if !cfg.UseKubernetesAPI() {
		return LocalSyncStatusReader{}
	}

	reader, err := NewFluxSyncStatusReader(cfg)
	if err != nil {
		return ErrorSyncStatusReader{Err: err}
	}

	return reader
}

func NewFluxSyncStatusReader(cfg core.Config) (FluxSyncStatusReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return FluxSyncStatusReader{}, err
	}

	return FluxSyncStatusReader{
		Client:                 client,
		KustomizationNamespace: strings.TrimSpace(cfg.FluxKustomizationNamespace),
	}, nil
}

func NewArgoRolloutController(cfg core.Config) (ArgoRolloutController, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return ArgoRolloutController{}, err
	}
	return ArgoRolloutController{Client: client}, nil
}

func NewPodMetricsReader(cfg core.Config) (PodMetricsReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return PodMetricsReader{}, err
	}

	return PodMetricsReader{
		Client: client,
		Range:  cfg.PrometheusRange,
		Step:   cfg.PrometheusStep,
	}, nil
}

func NewPodLogReader(cfg core.Config) (PodLogReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return PodLogReader{}, err
	}

	return PodLogReader{Client: client}, nil
}

func NewServiceNetworkExposureReader(cfg core.Config) (ServiceNetworkExposureReader, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return ServiceNetworkExposureReader{}, err
	}

	return ServiceNetworkExposureReader{Client: client}, nil
}

func (r FluxSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}
	if r.Client == nil {
		return application.SyncInfo{}, fmt.Errorf("kubernetes api client is not configured")
	}

	response, err := r.listKustomizations(ctx)
	if err != nil {
		return application.SyncInfo{}, err
	}

	item, ok := selectKustomization(response.Items, record)
	if !ok {
		now := time.Now().UTC()
		return application.SyncInfo{
			Status:     application.SyncStatusUnknown,
			Message:    fmt.Sprintf("Flux Kustomization for %s was not found.", desiredFluxPath(record)),
			ObservedAt: now,
		}, nil
	}

	return mapSyncInfo(item), nil
}

func (r FluxSyncStatusReader) ReadMany(ctx context.Context, records []application.Record) (map[string]application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return map[string]application.SyncInfo{}, nil
	}
	if r.Client == nil {
		return nil, fmt.Errorf("kubernetes api client is not configured")
	}

	response, err := r.listKustomizations(ctx)
	if err != nil {
		return nil, err
	}

	items := make(map[string]application.SyncInfo, len(records))
	for _, record := range records {
		item, ok := selectKustomization(response.Items, record)
		if !ok {
			now := time.Now().UTC()
			items[record.ID] = application.SyncInfo{
				Status:     application.SyncStatusUnknown,
				Message:    fmt.Sprintf("Flux Kustomization for %s was not found.", desiredFluxPath(record)),
				ObservedAt: now,
			}
			continue
		}
		items[record.ID] = mapSyncInfo(item)
	}

	return items, nil
}

func (r FluxSyncStatusReader) listKustomizations(ctx context.Context) (kustomizationListResponse, error) {
	resourcePath := fluxKustomizationResourcePath + "/kustomizations"
	namespace := strings.TrimSpace(r.KustomizationNamespace)
	if namespace != "" {
		resourcePath = fluxKustomizationResourcePath + "/namespaces/" + url.PathEscape(namespace) + "/kustomizations"
	}

	var response kustomizationListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return kustomizationListResponse{}, err
	}

	return response, nil
}

func (r PodMetricsReader) Read(ctx context.Context, record application.Record, duration time.Duration, step time.Duration) ([]application.MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Client == nil {
		return nil, fmt.Errorf("kubernetes api client is not configured")
	}

	resourcePath := kubernetesPodMetricsResourcePath + "/namespaces/" + url.PathEscape(record.Namespace) + "/pods"
	var response podMetricsListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return nil, err
	}

	now := r.now().UTC()

	queryStep := step
	if queryStep <= 0 {
		queryStep = r.metricStep()
	}
	queryWindow := duration
	if queryWindow <= 0 {
		queryWindow = r.metricRange()
	}

	end := now.Truncate(queryStep)
	start := end.Add(-queryWindow).Add(queryStep)
	if start.After(end) {
		start = end
	}

	cpuUsage, memoryUsage, found, err := collectPodResourceUsage(response.Items, record.Name)
	if err != nil {
		return nil, err
	}

	metrics := []application.MetricSeries{
		buildKubernetesMetricSeries("request_rate", "Requests", "rpm", start, end, queryStep, nil),
		buildKubernetesMetricSeries("error_rate", "Error Rate", "%", start, end, queryStep, nil),
		buildKubernetesMetricSeries("latency_p95", "P95 Latency", "ms", start, end, queryStep, nil),
	}

	if found {
		metrics = append(metrics,
			buildKubernetesMetricSeries("cpu_usage", "CPU Usage", "cores", start, end, queryStep, &cpuUsage),
			buildKubernetesMetricSeries("memory_usage", "Memory Usage", "MiB", start, end, queryStep, &memoryUsage),
		)
		return metrics, nil
	}

	metrics = append(metrics,
		buildKubernetesMetricSeries("cpu_usage", "CPU Usage", "cores", start, end, queryStep, nil),
		buildKubernetesMetricSeries("memory_usage", "Memory Usage", "MiB", start, end, queryStep, nil),
	)
	return metrics, nil
}

type podMetricsListResponse struct {
	Items []podMetricsResponse `json:"items"`
}

type podListResponse struct {
	Items []podResponse `json:"items"`
}

type podResponse struct {
	Metadata struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		CreationTimestamp time.Time         `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Containers []podSpecContainer `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase             string               `json:"phase"`
		ContainerStatuses []podContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type podSpecContainer struct {
	Name      string                `json:"name"`
	Resources podContainerResources `json:"resources"`
}

type podContainerResources struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

type podContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
}

type podMetricsResponse struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Containers []podMetricsContainer `json:"containers"`
}

type podMetricsContainer struct {
	Name  string `json:"name"`
	Usage struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"usage"`
}

type podContainerUsage struct {
	CPUCores  *float64
	MemoryMiB *float64
}

type serviceResponse struct {
	Spec struct {
		Type  string        `json:"type"`
		Ports []servicePort `json:"ports"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []serviceIngress `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

type serviceIngress struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

type servicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort any    `json:"targetPort"`
	NodePort   int    `json:"nodePort"`
}

type eventListResponse struct {
	Items []kubernetesEvent `json:"items"`
}

type kubernetesEvent struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	LastTimestamp string `json:"lastTimestamp"`
	EventTime     string `json:"eventTime"`
	Metadata      struct {
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
}

func collectPodResourceUsage(items []podMetricsResponse, appName string) (float64, float64, bool, error) {
	podPattern := regexp.MustCompile("^" + regexp.QuoteMeta(appName) + `-[a-z0-9]+(?:-[a-z0-9]+)?$`)

	var cpuUsage float64
	var memoryUsage float64
	found := false

	for _, item := range items {
		if !podPattern.MatchString(strings.TrimSpace(item.Metadata.Name)) {
			continue
		}
		found = true
		for _, container := range item.Containers {
			cpuValue, err := parseCPUQuantityToCores(container.Usage.CPU)
			if err != nil {
				return 0, 0, false, err
			}
			memoryValue, err := parseMemoryQuantityToMiB(container.Usage.Memory)
			if err != nil {
				return 0, 0, false, err
			}
			cpuUsage += cpuValue
			memoryUsage += memoryValue
		}
	}

	return cpuUsage, memoryUsage, found, nil
}

func (r ServiceNetworkExposureReader) Read(ctx context.Context, record application.Record) (application.NetworkExposureInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.NetworkExposureInfo{}, err
	}

	observedAt := time.Now().UTC()
	if !record.LoadBalancerEnabled {
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusInternal,
			Message:     "현재는 내부 전용(ClusterIP) 서비스로 운영 중입니다.",
			ServiceType: "ClusterIP",
			ObservedAt:  observedAt,
		}, nil
	}
	if r.Client == nil {
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusError,
			Message:     "Kubernetes API 클라이언트가 설정되지 않아 LoadBalancer 상태를 조회할 수 없습니다.",
			ServiceType: "LoadBalancer",
			ObservedAt:  observedAt,
		}, nil
	}

	service, err := r.getService(ctx, record)
	if err != nil {
		var apiErr apiRequestError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return application.NetworkExposureInfo{
				Status:      application.NetworkExposureStatusPending,
				Message:     "Service가 아직 생성되지 않았습니다. GitOps 반영이 끝나면 LoadBalancer 준비 단계가 시작됩니다.",
				ServiceType: "LoadBalancer",
				ObservedAt:  observedAt,
			}, nil
		}
		return application.NetworkExposureInfo{}, err
	}

	lastEvent, eventErr := r.getLatestServiceEvent(ctx, record)
	if eventErr == nil && lastEvent != nil && !lastEvent.ObservedAt.IsZero() {
		observedAt = lastEvent.ObservedAt
	}

	serviceType := strings.TrimSpace(service.Spec.Type)
	if serviceType == "" {
		serviceType = "ClusterIP"
	}
	addresses := collectServiceIngressAddresses(service.Status.LoadBalancer.Ingress)
	ports := collectServicePorts(service.Spec.Ports)

	switch {
	case strings.EqualFold(serviceType, "LoadBalancer") && len(addresses) > 0:
		message := fmt.Sprintf("클러스터 LoadBalancer 주소가 준비되었습니다: %s", strings.Join(addresses, ", "))
		if lastEvent != nil && strings.TrimSpace(lastEvent.Message) != "" {
			message = fmt.Sprintf("%s 최근 이벤트: %s", message, strings.TrimSpace(lastEvent.Message))
		}
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusReady,
			Message:     message,
			ServiceType: serviceType,
			Addresses:   addresses,
			Ports:       ports,
			LastEvent:   lastEvent,
			ObservedAt:  observedAt,
		}, nil
	case lastEvent != nil && strings.EqualFold(strings.TrimSpace(lastEvent.Type), "Warning"):
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusError,
			Message:     fmt.Sprintf("LoadBalancer 준비 중 경고가 발생했습니다. %s", strings.TrimSpace(lastEvent.Message)),
			ServiceType: serviceType,
			Ports:       ports,
			LastEvent:   lastEvent,
			ObservedAt:  observedAt,
		}, nil
	case !strings.EqualFold(serviceType, "LoadBalancer"):
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusPending,
			Message:     fmt.Sprintf("현재 Service 타입은 %s 입니다. 아직 LoadBalancer 반영 전입니다.", serviceType),
			ServiceType: serviceType,
			Ports:       ports,
			LastEvent:   lastEvent,
			ObservedAt:  observedAt,
		}, nil
	default:
		message := "Service는 LoadBalancer로 반영됐지만 외부 주소가 아직 할당되지 않았습니다."
		if lastEvent != nil && strings.TrimSpace(lastEvent.Message) != "" {
			message = fmt.Sprintf("%s 최근 이벤트: %s", message, strings.TrimSpace(lastEvent.Message))
		}
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusProvisioning,
			Message:     message,
			ServiceType: serviceType,
			Ports:       ports,
			LastEvent:   lastEvent,
			ObservedAt:  observedAt,
		}, nil
	}
}

func (r ServiceNetworkExposureReader) getService(ctx context.Context, record application.Record) (serviceResponse, error) {
	resourcePath := "/api/v1/namespaces/" + url.PathEscape(record.Namespace) + "/services/" + url.PathEscape(record.Name)
	var response serviceResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return serviceResponse{}, err
	}
	return response, nil
}

func (r ServiceNetworkExposureReader) getLatestServiceEvent(ctx context.Context, record application.Record) (*application.NetworkExposureEvent, error) {
	resourcePath := "/api/v1/namespaces/" + url.PathEscape(record.Namespace) +
		"/events?fieldSelector=" + url.QueryEscape("involvedObject.kind=Service,involvedObject.name="+record.Name)

	var response eventListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		var apiErr apiRequestError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, nil
	}

	sort.SliceStable(response.Items, func(i, j int) bool {
		return eventObservedAt(response.Items[i]).After(eventObservedAt(response.Items[j]))
	})

	selected := response.Items[0]
	return &application.NetworkExposureEvent{
		Type:       strings.TrimSpace(selected.Type),
		Reason:     strings.TrimSpace(selected.Reason),
		Message:    strings.TrimSpace(selected.Message),
		ObservedAt: eventObservedAt(selected),
	}, nil
}

func collectServiceIngressAddresses(items []serviceIngress) []string {
	seen := map[string]struct{}{}
	addresses := make([]string, 0, len(items))
	for _, item := range items {
		for _, value := range []string{strings.TrimSpace(item.IP), strings.TrimSpace(item.Hostname)} {
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			addresses = append(addresses, value)
		}
	}
	return addresses
}

func collectServicePorts(items []servicePort) []application.NetworkExposurePort {
	ports := make([]application.NetworkExposurePort, 0, len(items))
	for _, item := range items {
		if item.Port <= 0 {
			continue
		}
		ports = append(ports, application.NetworkExposurePort{
			Name:       strings.TrimSpace(item.Name),
			Protocol:   strings.TrimSpace(item.Protocol),
			Port:       item.Port,
			TargetPort: normalizeServiceTargetPort(item.TargetPort),
			NodePort:   item.NodePort,
		})
	}
	return ports
}

func normalizeServiceTargetPort(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.Itoa(int(typed))
	case int:
		return strconv.Itoa(typed)
	case int32:
		return strconv.Itoa(int(typed))
	case int64:
		return strconv.Itoa(int(typed))
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func eventObservedAt(item kubernetesEvent) time.Time {
	for _, raw := range []string{
		strings.TrimSpace(item.LastTimestamp),
		strings.TrimSpace(item.EventTime),
		strings.TrimSpace(item.Metadata.CreationTimestamp),
	} {
		if raw == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil && !parsed.IsZero() {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil && !parsed.IsZero() {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func (r PodLogReader) Read(ctx context.Context, record application.Record, tailLines int) ([]application.ContainerLogStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Client == nil {
		return nil, fmt.Errorf("kubernetes api client is not configured")
	}

	pods, err := r.listApplicationPods(ctx, record)
	if err != nil {
		return nil, err
	}
	if len(pods) > 2 {
		pods = pods[:2]
	}

	streams := make([]application.ContainerLogStream, 0, len(pods))
	for _, pod := range pods {
		containerName, status := selectPrimaryContainer(pod)
		if containerName == "" {
			continue
		}

		logPath := buildPodLogResourcePath(record.Namespace, pod.Metadata.Name, containerName, tailLines, false)
		content, err := r.Client.GetText(ctx, logPath)
		if err != nil {
			return nil, err
		}

		streams = append(streams, application.ContainerLogStream{
			PodName:       pod.Metadata.Name,
			ContainerName: containerName,
			Phase:         pod.Status.Phase,
			Ready:         status.Ready,
			RestartCount:  status.RestartCount,
			Content:       strings.TrimSpace(content),
		})
	}

	return streams, nil
}

func (r PodLogReader) ListTargets(ctx context.Context, record application.Record) ([]application.ContainerLogTarget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Client == nil {
		return nil, fmt.Errorf("kubernetes api client is not configured")
	}

	pods, err := r.listApplicationPods(ctx, record)
	if err != nil {
		return nil, err
	}

	usageByPod, err := r.readPodContainerUsage(ctx, record.Namespace)
	if err != nil {
		usageByPod = map[string]map[string]podContainerUsage{}
	}

	targets := make([]application.ContainerLogTarget, 0, len(pods))
	for _, pod := range pods {
		defaultContainerName, _ := selectPrimaryContainer(pod)
		statusByName := make(map[string]podContainerStatus, len(pod.Status.ContainerStatuses))
		for _, status := range pod.Status.ContainerStatuses {
			statusByName[status.Name] = status
		}

		containers := make([]application.ContainerLogTargetContainer, 0, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			status := statusByName[container.Name]
			resourceStatus, err := buildContainerResourceStatus(container.Resources, usageByPod[pod.Metadata.Name][container.Name])
			if err != nil {
				return nil, err
			}
			containers = append(containers, application.ContainerLogTargetContainer{
				Name:           container.Name,
				Ready:          status.Ready,
				RestartCount:   status.RestartCount,
				Default:        container.Name == defaultContainerName,
				ResourceStatus: resourceStatus,
			})
		}
		if len(containers) == 0 {
			continue
		}

		targets = append(targets, application.ContainerLogTarget{
			PodName:    pod.Metadata.Name,
			Phase:      pod.Status.Phase,
			Containers: containers,
		})
	}

	return targets, nil
}

func (r PodLogReader) Stream(
	ctx context.Context,
	record application.Record,
	podName string,
	containerName string,
	tailLines int,
	emit func(application.ContainerLogEvent) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Client == nil {
		return fmt.Errorf("kubernetes api client is not configured")
	}

	logPath := buildPodLogResourcePath(record.Namespace, podName, containerName, tailLines, true)
	return r.Client.StreamText(ctx, logPath, func(line string) error {
		return emit(buildContainerLogEvent(podName, containerName, line))
	})
}

func (r PodLogReader) listApplicationPods(ctx context.Context, record application.Record) ([]podResponse, error) {
	selector := url.QueryEscape("app.kubernetes.io/name=" + record.Name)
	resourcePath := "/api/v1/namespaces/" + url.PathEscape(record.Namespace) + "/pods?labelSelector=" + selector
	var response podListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return nil, err
	}
	return filterAndSortApplicationPods(response.Items, record.Name), nil
}

func (r PodLogReader) readPodContainerUsage(ctx context.Context, namespace string) (map[string]map[string]podContainerUsage, error) {
	resourcePath := kubernetesPodMetricsResourcePath + "/namespaces/" + url.PathEscape(namespace) + "/pods"
	var response podMetricsListResponse
	if err := r.Client.GetJSON(ctx, resourcePath, &response); err != nil {
		return nil, err
	}

	usageByPod := make(map[string]map[string]podContainerUsage, len(response.Items))
	for _, item := range response.Items {
		containers := make(map[string]podContainerUsage, len(item.Containers))
		for _, container := range item.Containers {
			cpuValue, err := parseCPUQuantityToCores(container.Usage.CPU)
			if err != nil {
				return nil, err
			}
			memoryValue, err := parseMemoryQuantityToMiB(container.Usage.Memory)
			if err != nil {
				return nil, err
			}
			cpuCopy := cpuValue
			memoryCopy := memoryValue
			containers[container.Name] = podContainerUsage{
				CPUCores:  &cpuCopy,
				MemoryMiB: &memoryCopy,
			}
		}
		usageByPod[item.Metadata.Name] = containers
	}

	return usageByPod, nil
}

func buildContainerResourceStatus(resources podContainerResources, usage podContainerUsage) (*application.ContainerResourceStatus, error) {
	status := &application.ContainerResourceStatus{
		CPUUsageCores:  usage.CPUCores,
		MemoryUsageMiB: usage.MemoryMiB,
	}

	if value, ok := resources.Requests["cpu"]; ok && strings.TrimSpace(value) != "" {
		parsed, err := parseCPUQuantityToCores(value)
		if err != nil {
			return nil, err
		}
		status.CPURequestCores = float64Pointer(parsed)
	}
	if value, ok := resources.Limits["cpu"]; ok && strings.TrimSpace(value) != "" {
		parsed, err := parseCPUQuantityToCores(value)
		if err != nil {
			return nil, err
		}
		status.CPULimitCores = float64Pointer(parsed)
	}
	if value, ok := resources.Requests["memory"]; ok && strings.TrimSpace(value) != "" {
		parsed, err := parseMemoryQuantityToMiB(value)
		if err != nil {
			return nil, err
		}
		status.MemoryRequestMiB = float64Pointer(parsed)
	}
	if value, ok := resources.Limits["memory"]; ok && strings.TrimSpace(value) != "" {
		parsed, err := parseMemoryQuantityToMiB(value)
		if err != nil {
			return nil, err
		}
		status.MemoryLimitMiB = float64Pointer(parsed)
	}

	status.CPURequestUtilization = utilization(status.CPUUsageCores, status.CPURequestCores)
	status.CPULimitUtilization = utilization(status.CPUUsageCores, status.CPULimitCores)
	status.MemoryRequestUtilization = utilization(status.MemoryUsageMiB, status.MemoryRequestMiB)
	status.MemoryLimitUtilization = utilization(status.MemoryUsageMiB, status.MemoryLimitMiB)

	if status.CPUUsageCores == nil &&
		status.CPURequestCores == nil &&
		status.CPULimitCores == nil &&
		status.MemoryUsageMiB == nil &&
		status.MemoryRequestMiB == nil &&
		status.MemoryLimitMiB == nil {
		return nil, nil
	}

	return status, nil
}

func buildPodLogResourcePath(namespace string, podName string, containerName string, tailLines int, follow bool) string {
	query := url.Values{}
	query.Set("container", containerName)
	query.Set("tailLines", strconv.Itoa(tailLines))
	query.Set("timestamps", "true")
	if follow {
		query.Set("follow", "true")
	}
	return "/api/v1/namespaces/" + url.PathEscape(namespace) + "/pods/" + url.PathEscape(podName) + "/log?" + query.Encode()
}

func buildContainerLogEvent(podName string, containerName string, rawLine string) application.ContainerLogEvent {
	trimmed := strings.TrimRight(rawLine, "\r")
	timestamp := ""
	message := trimmed
	if first, rest, ok := strings.Cut(trimmed, " "); ok {
		if _, err := time.Parse(time.RFC3339Nano, first); err == nil {
			timestamp = first
			message = rest
		}
	}

	return application.ContainerLogEvent{
		PodName:       podName,
		ContainerName: containerName,
		Timestamp:     timestamp,
		Message:       message,
		RawLine:       trimmed,
	}
}

func float64Pointer(value float64) *float64 {
	copy := value
	return &copy
}

func utilization(used *float64, allocated *float64) *float64 {
	if used == nil || allocated == nil || *allocated <= 0 {
		return nil
	}
	value := (*used / *allocated) * 100
	return &value
}

func filterAndSortApplicationPods(items []podResponse, appName string) []podResponse {
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(appName) + `-[a-z0-9]+(?:-[a-z0-9]+)?$`)
	filtered := make([]podResponse, 0, len(items))
	for _, item := range items {
		if pattern.MatchString(strings.TrimSpace(item.Metadata.Name)) {
			filtered = append(filtered, item)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		if left.Status.Phase == "Running" && right.Status.Phase != "Running" {
			return true
		}
		if left.Status.Phase != "Running" && right.Status.Phase == "Running" {
			return false
		}
		return left.Metadata.CreationTimestamp.After(right.Metadata.CreationTimestamp)
	})

	return filtered
}

func selectPrimaryContainer(pod podResponse) (string, podContainerStatus) {
	statusByName := make(map[string]podContainerStatus, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		statusByName[status.Name] = status
	}

	for _, container := range pod.Spec.Containers {
		if isSidecarContainer(container.Name) {
			continue
		}
		return container.Name, statusByName[container.Name]
	}

	if len(pod.Spec.Containers) == 0 {
		return "", podContainerStatus{}
	}
	first := pod.Spec.Containers[0]
	return first.Name, statusByName[first.Name]
}

func isSidecarContainer(name string) bool {
	switch strings.TrimSpace(name) {
	case "istio-proxy", "linkerd-proxy", "vault-agent":
		return true
	default:
		return false
	}
}

func buildKubernetesMetricSeries(
	key string,
	label string,
	unit string,
	start time.Time,
	end time.Time,
	step time.Duration,
	latestValue *float64,
) application.MetricSeries {
	points := make([]application.MetricPoint, 0, int(end.Sub(start)/step)+1)
	for current := start; !current.After(end); current = current.Add(step) {
		point := application.MetricPoint{Timestamp: current}
		if latestValue != nil && current.Equal(end) {
			valueCopy := *latestValue
			point.Value = &valueCopy
		}
		points = append(points, point)
	}

	return application.MetricSeries{
		Key:    key,
		Label:  label,
		Unit:   unit,
		Points: points,
	}
}

func parseCPUQuantityToCores(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	for _, unit := range []struct {
		Suffix     string
		Multiplier float64
	}{
		{Suffix: "n", Multiplier: 1e-9},
		{Suffix: "u", Multiplier: 1e-6},
		{Suffix: "m", Multiplier: 1e-3},
	} {
		if strings.HasSuffix(value, unit.Suffix) {
			number, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.Suffix), 64)
			if err != nil {
				return 0, fmt.Errorf("parse kubernetes cpu quantity %q: %w", raw, err)
			}
			return number * unit.Multiplier, nil
		}
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse kubernetes cpu quantity %q: %w", raw, err)
	}
	return number, nil
}

func parseMemoryQuantityToMiB(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	for _, unit := range []struct {
		Suffix     string
		Multiplier float64
	}{
		{Suffix: "Ki", Multiplier: 1 / 1024.0},
		{Suffix: "Mi", Multiplier: 1},
		{Suffix: "Gi", Multiplier: 1024},
		{Suffix: "Ti", Multiplier: 1024 * 1024},
		{Suffix: "Pi", Multiplier: 1024 * 1024 * 1024},
		{Suffix: "Ei", Multiplier: 1024 * 1024 * 1024 * 1024},
		{Suffix: "K", Multiplier: 1000 / (1024.0 * 1024.0)},
		{Suffix: "M", Multiplier: 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "G", Multiplier: 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "T", Multiplier: 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "P", Multiplier: 1000 * 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "E", Multiplier: 1000 * 1000 * 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
	} {
		if strings.HasSuffix(value, unit.Suffix) {
			number, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.Suffix), 64)
			if err != nil {
				return 0, fmt.Errorf("parse kubernetes memory quantity %q: %w", raw, err)
			}
			return number * unit.Multiplier, nil
		}
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse kubernetes memory quantity %q: %w", raw, err)
	}
	return number / (1024.0 * 1024.0), nil
}

func (r PodMetricsReader) metricRange() time.Duration {
	if r.Range > 0 {
		return r.Range
	}
	return time.Hour
}

func (r PodMetricsReader) metricStep() time.Duration {
	if r.Step > 0 {
		return r.Step
	}
	return 5 * time.Minute
}

func (r PodMetricsReader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

type apiClient struct {
	BaseURL             string
	BearerTokenProvider func(context.Context) (string, error)
	HTTPClient          *http.Client
}

type apiRequestError struct {
	ResourcePath string
	StatusCode   int
	Message      string
}

func (e apiRequestError) Error() string {
	return fmt.Sprintf("kubernetes api %s failed with status %d: %s", e.ResourcePath, e.StatusCode, e.Message)
}

func newAPIClient(cfg core.Config) (*apiClient, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.KubernetesMode)) {
	case "", "local":
		return nil, fmt.Errorf("kubernetes api mode is local")
	case "token":
		return newTokenAPIClient(cfg)
	case "kubeconfig":
		return newKubeconfigAPIClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported kubernetes mode %q", cfg.KubernetesMode)
	}
}

func newTokenAPIClient(cfg core.Config) (*apiClient, error) {
	baseURL := strings.TrimSpace(cfg.KubernetesAPIURL)
	if baseURL == "" {
		return nil, fmt.Errorf("AODS_K8S_API_URL is required when AODS_K8S_MODE=token")
	}
	token := strings.TrimSpace(cfg.KubernetesBearerToken)
	if token == "" {
		return nil, fmt.Errorf("AODS_K8S_BEARER_TOKEN is required when AODS_K8S_MODE=token")
	}

	httpClient, err := newHTTPClient(cfg.KubernetesRequestTimeout, cfg.KubernetesCAFile, cfg.KubernetesCAData, false, nil)
	if err != nil {
		return nil, err
	}

	return &apiClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		BearerTokenProvider: func(context.Context) (string, error) {
			return token, nil
		},
		HTTPClient: httpClient,
	}, nil
}

func newKubeconfigAPIClient(cfg core.Config) (*apiClient, error) {
	configPath := strings.TrimSpace(cfg.KubernetesKubeconfigPath)
	if configPath == "" {
		return nil, fmt.Errorf("AODS_K8S_KUBECONFIG is required when AODS_K8S_MODE=kubeconfig")
	}
	configPath = filepath.Clean(configPath)
	configDir := filepath.Dir(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig: %w", err)
	}

	var kubeconfig kubeconfigDocument
	if err := yaml.Unmarshal(data, &kubeconfig); err != nil {
		return nil, fmt.Errorf("decode kubeconfig: %w", err)
	}

	selectedContextName := strings.TrimSpace(cfg.KubernetesContext)
	if selectedContextName == "" {
		selectedContextName = strings.TrimSpace(kubeconfig.CurrentContext)
	}
	if selectedContextName == "" {
		return nil, fmt.Errorf("kubeconfig current-context is empty")
	}

	selectedContext, err := kubeconfig.contextByName(selectedContextName)
	if err != nil {
		return nil, err
	}
	selectedCluster, err := kubeconfig.clusterByName(selectedContext.Context.Cluster)
	if err != nil {
		return nil, err
	}
	selectedUser, err := kubeconfig.userByName(selectedContext.Context.User)
	if err != nil {
		return nil, err
	}

	clientCertificate, err := selectedUser.User.resolveClientCertificate(configDir)
	if err != nil {
		return nil, err
	}
	bearerTokenProvider := selectedUser.User.resolveBearerTokenProvider(configDir)
	if bearerTokenProvider == nil && clientCertificate == nil {
		return nil, fmt.Errorf("kubeconfig user does not provide exec, token, token-file, or client certificate credentials")
	}

	httpClient, err := newHTTPClient(
		cfg.KubernetesRequestTimeout,
		resolveKubeconfigPath(configDir, selectedCluster.Cluster.CertificateAuthority),
		selectedCluster.Cluster.CertificateAuthorityData,
		selectedCluster.Cluster.InsecureSkipTLSVerify,
		clientCertificate,
	)
	if err != nil {
		return nil, err
	}

	return &apiClient{
		BaseURL:             strings.TrimRight(selectedCluster.Cluster.Server, "/"),
		BearerTokenProvider: bearerTokenProvider,
		HTTPClient:          httpClient,
	}, nil
}

func newHTTPClient(
	timeout time.Duration,
	caFile string,
	caData string,
	insecureSkipTLSVerify bool,
	clientCertificate *tls.Certificate,
) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipTLSVerify,
	}
	if clientCertificate != nil {
		transport.TLSClientConfig.Certificates = []tls.Certificate{*clientCertificate}
	}

	if !insecureSkipTLSVerify {
		rootCAs, err := x509.SystemCertPool()
		if err != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		if strings.TrimSpace(caFile) != "" {
			pem, err := os.ReadFile(caFile)
			if err != nil {
				return nil, fmt.Errorf("read kubernetes CA file: %w", err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("append kubernetes CA file: no certificates found")
			}
		}

		if strings.TrimSpace(caData) != "" {
			pem, err := base64.StdEncoding.DecodeString(caData)
			if err != nil {
				return nil, fmt.Errorf("decode kubernetes CA data: %w", err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("append kubernetes CA data: no certificates found")
			}
		}

		transport.TLSClientConfig.RootCAs = rootCAs
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

func (c *apiClient) GetJSON(ctx context.Context, resourcePath string, target any) error {
	return c.doJSON(ctx, http.MethodGet, resourcePath, nil, "", target)
}

func (c *apiClient) GetText(ctx context.Context, resourcePath string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kubernetes api client is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+resourcePath, nil)
	if err != nil {
		return "", fmt.Errorf("create kubernetes api request: %w", err)
	}
	if c.BearerTokenProvider != nil {
		token, err := c.BearerTokenProvider(ctx)
		if err != nil {
			return "", fmt.Errorf("resolve kubernetes api bearer token: %w", err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("perform kubernetes api request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read kubernetes api response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = response.Status
		}
		return "", apiRequestError{ResourcePath: resourcePath, StatusCode: response.StatusCode, Message: message}
	}

	return string(responseBody), nil
}

func (c *apiClient) StreamText(ctx context.Context, resourcePath string, onLine func(string) error) error {
	if c == nil {
		return fmt.Errorf("kubernetes api client is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+resourcePath, nil)
	if err != nil {
		return fmt.Errorf("create kubernetes api request: %w", err)
	}
	if c.BearerTokenProvider != nil {
		token, err := c.BearerTokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("resolve kubernetes api bearer token: %w", err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}

	response, err := c.streamingHTTPClient().Do(request)
	if err != nil {
		return fmt.Errorf("perform kubernetes api request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return apiRequestError{ResourcePath: resourcePath, StatusCode: response.StatusCode, Message: message}
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := onLine(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read kubernetes api stream: %w", err)
	}

	return nil
}

func (c *apiClient) streamingHTTPClient() *http.Client {
	if c == nil || c.HTTPClient == nil {
		return http.DefaultClient
	}
	clone := *c.HTTPClient
	clone.Timeout = 0
	return &clone
}

func (c *apiClient) PatchJSON(ctx context.Context, resourcePath string, body []byte, target any) error {
	return c.doJSON(ctx, http.MethodPatch, resourcePath, body, "application/merge-patch+json", target)
}

func (c *apiClient) doJSON(ctx context.Context, method string, resourcePath string, body []byte, contentType string, target any) error {
	if c == nil {
		return fmt.Errorf("kubernetes api client is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+resourcePath, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create kubernetes api request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if c.BearerTokenProvider != nil {
		token, err := c.BearerTokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("resolve kubernetes api bearer token: %w", err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform kubernetes api request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read kubernetes api response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = response.Status
		}
		return apiRequestError{ResourcePath: resourcePath, StatusCode: response.StatusCode, Message: message}
	}

	if target == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode kubernetes api response: %w", err)
	}

	return nil
}

type kubeconfigDocument struct {
	CurrentContext string               `yaml:"current-context"`
	Clusters       []namedKubeCluster   `yaml:"clusters"`
	Contexts       []namedKubeContext   `yaml:"contexts"`
	Users          []namedKubeUserEntry `yaml:"users"`
}

type namedKubeCluster struct {
	Name    string      `yaml:"name"`
	Cluster kubeCluster `yaml:"cluster"`
}

type kubeCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthority     string `yaml:"certificate-authority"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
}

type namedKubeContext struct {
	Name    string      `yaml:"name"`
	Context kubeContext `yaml:"context"`
}

type kubeContext struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type namedKubeUserEntry struct {
	Name string   `yaml:"name"`
	User kubeUser `yaml:"user"`
}

type kubeUser struct {
	Token                 string           `yaml:"token"`
	TokenFile             string           `yaml:"token-file"`
	ClientCertificate     string           `yaml:"client-certificate"`
	ClientCertificateData string           `yaml:"client-certificate-data"`
	ClientKey             string           `yaml:"client-key"`
	ClientKeyData         string           `yaml:"client-key-data"`
	Exec                  *kubeExecCommand `yaml:"exec"`
}

type kubeExecCommand struct {
	Command string        `yaml:"command"`
	Args    []string      `yaml:"args"`
	Env     []kubeExecEnv `yaml:"env"`
}

type kubeExecEnv struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type execCredentialResponse struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

func (c kubeconfigDocument) contextByName(name string) (namedKubeContext, error) {
	for _, item := range c.Contexts {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeContext{}, fmt.Errorf("kubeconfig context %q was not found", name)
}

func (c kubeconfigDocument) clusterByName(name string) (namedKubeCluster, error) {
	for _, item := range c.Clusters {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeCluster{}, fmt.Errorf("kubeconfig cluster %q was not found", name)
}

func (c kubeconfigDocument) userByName(name string) (namedKubeUserEntry, error) {
	for _, item := range c.Users {
		if item.Name == name {
			return item, nil
		}
	}
	return namedKubeUserEntry{}, fmt.Errorf("kubeconfig user %q was not found", name)
}

func (u kubeUser) resolveBearerTokenProvider(baseDir string) func(context.Context) (string, error) {
	if strings.TrimSpace(u.Token) != "" {
		token := strings.TrimSpace(u.Token)
		return func(context.Context) (string, error) {
			return token, nil
		}
	}

	if strings.TrimSpace(u.TokenFile) != "" {
		tokenFile := resolveKubeconfigPath(baseDir, u.TokenFile)
		return func(context.Context) (string, error) {
			data, err := os.ReadFile(tokenFile)
			if err != nil {
				return "", fmt.Errorf("read kubeconfig token file: %w", err)
			}
			return strings.TrimSpace(string(data)), nil
		}
	}

	if u.Exec != nil {
		return func(ctx context.Context) (string, error) {
			return u.Exec.resolveToken(ctx, baseDir)
		}
	}

	return nil
}

func (u kubeUser) resolveClientCertificate(baseDir string) (*tls.Certificate, error) {
	certPEM, err := u.readClientCertificatePEM(baseDir)
	if err != nil {
		return nil, err
	}
	keyPEM, err := u.readClientKeyPEM(baseDir)
	if err != nil {
		return nil, err
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, nil
	}

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig client certificate: %w", err)
	}

	return &certificate, nil
}

func (u kubeUser) readClientCertificatePEM(baseDir string) ([]byte, error) {
	if strings.TrimSpace(u.ClientCertificateData) != "" {
		pem, err := base64.StdEncoding.DecodeString(strings.TrimSpace(u.ClientCertificateData))
		if err != nil {
			return nil, fmt.Errorf("decode kubeconfig client-certificate-data: %w", err)
		}
		return pem, nil
	}

	if strings.TrimSpace(u.ClientCertificate) != "" {
		pem, err := os.ReadFile(resolveKubeconfigPath(baseDir, u.ClientCertificate))
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig client-certificate: %w", err)
		}
		return pem, nil
	}

	return nil, nil
}

func (u kubeUser) readClientKeyPEM(baseDir string) ([]byte, error) {
	if strings.TrimSpace(u.ClientKeyData) != "" {
		pem, err := base64.StdEncoding.DecodeString(strings.TrimSpace(u.ClientKeyData))
		if err != nil {
			return nil, fmt.Errorf("decode kubeconfig client-key-data: %w", err)
		}
		return pem, nil
	}

	if strings.TrimSpace(u.ClientKey) != "" {
		pem, err := os.ReadFile(resolveKubeconfigPath(baseDir, u.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig client-key: %w", err)
		}
		return pem, nil
	}

	return nil, nil
}

func (e kubeExecCommand) resolveToken(ctx context.Context, baseDir string) (string, error) {
	if strings.TrimSpace(e.Command) == "" {
		return "", fmt.Errorf("kubeconfig exec command is empty")
	}

	cmd := exec.CommandContext(ctx, resolveExecCommand(baseDir, e.Command), e.Args...)
	cmd.Env = append(os.Environ(), renderExecEnv(e.Env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("execute kubeconfig auth command: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var credential execCredentialResponse
	if err := json.Unmarshal(output, &credential); err != nil {
		return "", fmt.Errorf("decode kubeconfig exec credential: %w", err)
	}
	if strings.TrimSpace(credential.Status.Token) == "" {
		return "", fmt.Errorf("kubeconfig exec credential did not include a token")
	}

	return strings.TrimSpace(credential.Status.Token), nil
}

func renderExecEnv(values []kubeExecEnv) []string {
	rendered := make([]string, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		rendered = append(rendered, item.Name+"="+item.Value)
	}
	return rendered
}

func resolveKubeconfigPath(baseDir string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "~") {
		return trimmed
	}
	return filepath.Join(baseDir, trimmed)
}

func resolveExecCommand(baseDir string, command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" || filepath.IsAbs(trimmed) || !strings.ContainsRune(trimmed, filepath.Separator) {
		return trimmed
	}
	return filepath.Join(baseDir, trimmed)
}

type kustomizationListResponse struct {
	Items []fluxKustomization `json:"items"`
}

type fluxKustomization struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Path            string `json:"path"`
		TargetNamespace string `json:"targetNamespace"`
	} `json:"spec"`
	Status struct {
		Conditions []fluxCondition `json:"conditions"`
	} `json:"status"`
}

type fluxCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

func selectKustomization(items []fluxKustomization, record application.Record) (fluxKustomization, bool) {
	desiredPathValue := desiredFluxPath(record)

	var matches []fluxKustomization
	for _, item := range items {
		if normalizeFluxPath(item.Spec.Path) == desiredPathValue {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return fluxKustomization{}, false
	}
	if len(matches) == 1 {
		return matches[0], true
	}

	for _, item := range matches {
		if strings.TrimSpace(item.Spec.TargetNamespace) == record.Namespace {
			return item, true
		}
	}

	return matches[0], true
}

func desiredFluxPath(record application.Record) string {
	environment := strings.TrimSpace(record.DefaultEnvironment)
	if environment == "" {
		environment = "shared"
	}
	return normalizeFluxPath(path.Join("apps", record.ProjectID, record.Name, "overlays", environment))
}

func normalizeFluxPath(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.TrimPrefix(trimmed, "/")
	trimmed = path.Clean(trimmed)
	if trimmed == "." {
		return ""
	}
	return trimmed
}

func mapSyncInfo(item fluxKustomization) application.SyncInfo {
	ready := findCondition(item.Status.Conditions, "Ready")
	reconciling := findCondition(item.Status.Conditions, "Reconciling")
	stalled := findCondition(item.Status.Conditions, "Stalled")

	switch {
	case conditionIsTrue(ready) && ready.Reason == "ReconciliationSucceeded":
		return buildSyncInfo(application.SyncStatusSynced, *ready)
	case conditionIsFalse(ready):
		return buildSyncInfo(application.SyncStatusDegraded, *ready)
	case conditionIsTrue(stalled):
		return buildSyncInfo(application.SyncStatusDegraded, *stalled)
	case conditionIsTrue(reconciling):
		return buildSyncInfo(application.SyncStatusSyncing, *reconciling)
	case conditionStatusEquals(ready, "Unknown"):
		return buildSyncInfo(application.SyncStatusSyncing, *ready)
	default:
		condition := firstNonEmptyCondition(ready, reconciling, stalled)
		if condition == nil {
			return application.SyncInfo{
				Status:     application.SyncStatusUnknown,
				Message:    "Flux Kustomization does not expose usable status conditions yet.",
				ObservedAt: time.Now().UTC(),
			}
		}
		return buildSyncInfo(application.SyncStatusUnknown, *condition)
	}
}

func buildSyncInfo(status application.SyncStatus, condition fluxCondition) application.SyncInfo {
	observedAt, err := time.Parse(time.RFC3339, condition.LastTransitionTime)
	if err != nil || observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	message := strings.TrimSpace(condition.Message)
	if message == "" {
		message = strings.TrimSpace(condition.Reason)
	}
	if message == "" {
		message = "Flux Kustomization status condition was read successfully."
	}

	return application.SyncInfo{
		Status:     status,
		Message:    message,
		ObservedAt: observedAt,
	}
}

func (c ArgoRolloutController) GetRollout(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	rollout, err := c.fetchRollout(ctx, record)
	if err != nil {
		return application.RolloutInfo{}, err
	}
	return mapRolloutInfo(rollout), nil
}

func (c ArgoRolloutController) Promote(ctx context.Context, record application.Record, full bool) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	rollout, err := c.fetchRollout(ctx, record)
	if err != nil {
		return application.RolloutInfo{}, err
	}

	specPatch, statusPatch, unifiedPatch := buildPromotePatches(rollout, full)
	resourcePath := rolloutResourcePath(record)
	if statusPatch != nil {
		if err := c.Client.PatchJSON(ctx, resourcePath+"/status", statusPatch, nil); err != nil {
			var apiErr apiRequestError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return application.RolloutInfo{}, err
			}
			specPatch = unifiedPatch
		}
	}
	if specPatch != nil {
		if err := c.Client.PatchJSON(ctx, resourcePath, specPatch, nil); err != nil {
			return application.RolloutInfo{}, err
		}
	}

	updated, err := c.fetchRollout(ctx, record)
	if err != nil {
		return application.RolloutInfo{}, err
	}
	return mapRolloutInfo(updated), nil
}

func (c ArgoRolloutController) Abort(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	resourcePath := rolloutResourcePath(record)
	abortPatch := []byte(`{"status":{"abort":true}}`)
	if err := c.Client.PatchJSON(ctx, resourcePath+"/status", abortPatch, nil); err != nil {
		var apiErr apiRequestError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			return application.RolloutInfo{}, err
		}
		if err := c.Client.PatchJSON(ctx, resourcePath, abortPatch, nil); err != nil {
			return application.RolloutInfo{}, err
		}
	}

	updated, err := c.fetchRollout(ctx, record)
	if err != nil {
		return application.RolloutInfo{}, err
	}
	return mapRolloutInfo(updated), nil
}

func (c ArgoRolloutController) fetchRollout(ctx context.Context, record application.Record) (rolloutResponse, error) {
	if c.Client == nil {
		return rolloutResponse{}, fmt.Errorf("kubernetes api client is not configured")
	}
	var rollout rolloutResponse
	if err := c.Client.GetJSON(ctx, rolloutResourcePath(record), &rollout); err != nil {
		return rolloutResponse{}, err
	}
	return rollout, nil
}

func rolloutResourcePath(record application.Record) string {
	return argoRolloutResourcePath + "/namespaces/" + url.PathEscape(record.Namespace) + "/rollouts/" + url.PathEscape(record.Name)
}

func buildPromotePatches(rollout rolloutResponse, full bool) ([]byte, []byte, []byte) {
	const (
		unpausePatch                      = `{"spec":{"paused":false}}`
		clearPauseConditionsPatch         = `{"status":{"pauseConditions":null}}`
		unpauseAndClearPauseConditions    = `{"spec":{"paused":false},"status":{"pauseConditions":null}}`
		promoteFullPatch                  = `{"status":{"promoteFull":true}}`
		unpauseAndPromoteFullPatch        = `{"spec":{"paused":false},"status":{"promoteFull":true}}`
		clearPauseConditionsPatchWithStep = `{"status":{"pauseConditions":null, "currentStepIndex":%d}}`
		unpauseAndClearWithStepPatch      = `{"spec":{"paused":false},"status":{"pauseConditions":null, "currentStepIndex":%d}}`
	)

	var specPatch []byte
	var statusPatch []byte
	var unifiedPatch []byte

	if full {
		if rollout.Spec.Paused {
			specPatch = []byte(unpausePatch)
		}
		if rollout.Status.CurrentPodHash != rollout.Status.StableRS {
			statusPatch = []byte(promoteFullPatch)
		}
		return specPatch, statusPatch, []byte(unpauseAndPromoteFullPatch)
	}

	unifiedPatch = []byte(unpauseAndClearPauseConditions)
	if rollout.Spec.Paused {
		specPatch = []byte(unpausePatch)
	}
	if len(rollout.Status.PauseConditions) > 0 {
		statusPatch = []byte(clearPauseConditionsPatch)
		return specPatch, statusPatch, unifiedPatch
	}

	if rollout.Spec.Strategy.Canary != nil && len(rollout.Spec.Strategy.Canary.Steps) > 0 {
		current := 0
		if rollout.Status.CurrentStepIndex != nil {
			current = *rollout.Status.CurrentStepIndex
		}
		if current < len(rollout.Spec.Strategy.Canary.Steps) {
			current++
		}
		statusPatch = []byte(fmt.Sprintf(clearPauseConditionsPatchWithStep, current))
		unifiedPatch = []byte(fmt.Sprintf(unpauseAndClearWithStepPatch, current))
	}

	return specPatch, statusPatch, unifiedPatch
}

type rolloutResponse struct {
	Spec struct {
		Paused   bool `json:"paused"`
		Strategy struct {
			Canary *struct {
				Steps []map[string]any `json:"steps"`
			} `json:"canary,omitempty"`
		} `json:"strategy"`
	} `json:"spec"`
	Status struct {
		Phase            string `json:"phase"`
		Message          string `json:"message"`
		CurrentStepIndex *int   `json:"currentStepIndex"`
		CurrentPodHash   string `json:"currentPodHash"`
		StableRS         string `json:"stableRS"`
		PauseConditions  []struct {
			Reason string `json:"reason"`
		} `json:"pauseConditions"`
		Canary struct {
			Weights *struct {
				Canary struct {
					Weight int `json:"weight"`
				} `json:"canary"`
			} `json:"weights,omitempty"`
		} `json:"canary"`
	} `json:"status"`
}

func mapRolloutInfo(rollout rolloutResponse) application.RolloutInfo {
	var currentStep *int
	if rollout.Status.CurrentStepIndex != nil {
		step := *rollout.Status.CurrentStepIndex + 1
		currentStep = &step
	}

	var canaryWeight *int
	if rollout.Status.Canary.Weights != nil {
		weight := rollout.Status.Canary.Weights.Canary.Weight
		canaryWeight = &weight
	}

	message := strings.TrimSpace(rollout.Status.Message)
	if message == "" && len(rollout.Status.PauseConditions) > 0 {
		message = rollout.Status.PauseConditions[0].Reason
	}
	if message == "" {
		message = "Argo Rollout 상태를 조회했습니다."
	}

	phase := strings.TrimSpace(rollout.Status.Phase)
	if phase == "" {
		phase = "Unknown"
	}

	return application.RolloutInfo{
		Phase:          phase,
		CurrentStep:    currentStep,
		CanaryWeight:   canaryWeight,
		StableRevision: rollout.Status.StableRS,
		CanaryRevision: rollout.Status.CurrentPodHash,
		Message:        message,
	}
}

func findCondition(conditions []fluxCondition, kind string) *fluxCondition {
	for _, item := range conditions {
		if item.Type == kind {
			condition := item
			return &condition
		}
	}
	return nil
}

func firstNonEmptyCondition(conditions ...*fluxCondition) *fluxCondition {
	for _, item := range conditions {
		if item != nil {
			return item
		}
	}
	return nil
}

func conditionIsTrue(condition *fluxCondition) bool {
	return conditionStatusEquals(condition, "True")
}

func conditionIsFalse(condition *fluxCondition) bool {
	return conditionStatusEquals(condition, "False")
}

func conditionStatusEquals(condition *fluxCondition, expected string) bool {
	return condition != nil && strings.EqualFold(strings.TrimSpace(condition.Status), expected)
}
