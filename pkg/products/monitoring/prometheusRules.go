package monitoring

import (
	"context"
	"fmt"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	integreatlyv1alpha1 "github.com/integr8ly/integreatly-operator/pkg/apis/integreatly/v1alpha1"
	"github.com/integr8ly/integreatly-operator/pkg/config"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (r *Reconciler) reconcileBackupMonitoringAlerts(ctx context.Context, serverClient k8sclient.Client) (integreatlyv1alpha1.StatusPhase, error) {
	monitoringConfig := config.NewMonitoring(config.ProductConfig{})
	rule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-monitoring-alerts",
			Namespace: r.Config.GetOperatorNamespace(),
		},
	}

	rules := []monitoringv1.Rule{
		{
			Alert: fmt.Sprintf("JobRunningTimeExceeded"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf(" Job {{ $labels.namespace }} / {{ $labels.job  }} has been running for longer than 300 seconds"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("time() - (max(kube_job_status_active * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'}) BY (job_name) * ON(job_name) GROUP_RIGHT() max(kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'}) BY (job_name, namespace, label_cronjob_name) > 0) > 300 ")),
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("JobRunningTimeExceeded"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf(" Job {{ $labels.namespace }} / {{ $labels.job  }} has been running for longer than 600 seconds"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("time() - (max(kube_job_status_active * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'}) BY (job_name) * ON(job_name) GROUP_RIGHT() max(kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'}) BY (job_name, namespace, label_cronjob_name) > 0) > 600 ")),
			For:    "5m",
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("CronJobSuspended"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf(" CronJob {{ $labels.namespace  }} / {{ $labels.cronjob }} is suspended"),
				},	
			Expr: intstr.FromString(fmt.Sprintf("kube_cronjob_labels{ label_monitoring_key='middleware' } * ON (cronjob) GROUP_RIGHT() kube_cronjob_spec_suspend > 0 ")),
			For:    "60s",
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("CronJobNotRunInThreshold"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf(" CronJob {{ $labels.namespace }} / {{ $labels.label_cronjob_name }} has not started a Job in 25 hours"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("(time() - (max( kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'} ) BY (job_name, label_cronjob_name) == ON(label_cronjob_name) GROUP_LEFT() max( kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'} ) BY (label_cronjob_name))) > 60*60*25")),
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("CronJobsFailed"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Job {{ $labels.namespace  }} / {{  $labels.job  }} has failed"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("clamp_max(max(kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'} ) BY (job_name, label_cronjob_name, namespace) == ON(label_cronjob_name) GROUP_LEFT() max(kube_job_status_start_time * ON(job_name) GROUP_RIGHT() kube_job_labels{label_monitoring_key='middleware'}) BY (label_cronjob_name), 1) * ON(job_name) GROUP_LEFT() kube_job_status_failed > 0")),
			For:    "5m",
			Labels: map[string]string{"severity": "critical"},
		}}
	_, err := controllerutil.CreateOrUpdate(ctx, serverClient, rule, func() error {
		rule.ObjectMeta.Labels = map[string]string{"integreatly": "yes", monitoringConfig.GetLabelSelectorKey(): monitoringConfig.GetLabelSelector()}
		rule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:  "general.rules",
					Rules: rules,
				},
			},
		}
		return nil
	})

	if err != nil {
		logrus.Infof("Phase: %s reconcilePrometheusAlerts", err)

		return integreatlyv1alpha1.PhaseFailed, fmt.Errorf("error creating backup PrometheusRule: %w", err)
	}
	return integreatlyv1alpha1.PhaseCompleted, nil
}



func (r *Reconciler) reconcileKubeStateMetricsAlerts(ctx context.Context, serverClient k8sclient.Client) (integreatlyv1alpha1.StatusPhase, error) {
	monitoringConfig := config.NewMonitoring(config.ProductConfig{})
	rule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ksm-alerts",
			Namespace: r.Config.GetOperatorNamespace(),
		},
	}

	rules := []monitoringv1.Rule{
		{
			Alert: fmt.Sprintf("KubePodCrashLooping"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Pod {{ $labels.namespace }} / {{  $labels.pod  }} ({{ $labels.container  }}) is restarting {{ $value }} times every 5 minutes; for the last 15 minutes."),
				},	
			Expr: intstr.FromString(fmt.Sprintf("rate(kube_pod_container_status_restarts_total{job='kube-state-metrics'}[15m]) * on (namespace, namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'} * 60 * 5 > 0")),
			For:    "15m",
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("KubePodNotReady"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Pod {{ $labels.namespace }} / {{ $labels.pod }}  has been in a non-ready state for longer than 15 minutes."),
				},	
			Expr: intstr.FromString(fmt.Sprintf("sum by(pod, namespace) (kube_pod_status_phase{phase=~'Pending|Unknown'} * on (namespace, namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) > 0")),
			For:    "15m",
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("KubePodImagePullBackOff"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Pod {{ $labels.namespace }} / {{  $labels.pod  }} has been unable to pull it's image for longer than 5 minutes."),
				},	
			Expr: intstr.FromString(fmt.Sprintf("(kube_pod_container_status_waiting_reason{reason='ImagePullBackOff'} * on (namespace, namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) > 0")),
			For:    "5m",
			Labels: map[string]string{"severity": "critical"},
		},
		{
			Alert: fmt.Sprintf("KubePodBadConfig"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf(" Pod {{ $labels.namespace  }} / {{  $labels.pod  }} has been unable to start due to a bad configuration for longer than 5 minutes"),
				},	
			Expr: intstr.FromString(fmt.Sprintf("(kube_pod_container_status_waiting_reason{reason='CreateContainerConfigError'} * on (namespace, namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) > 0")),
			For: "5m",
			Labels: map[string]string{"severity": "critical"},	
		},
		{
			Alert: fmt.Sprintf("KubePodStuckCreating"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Pod {{  $labels.namespace }} / {{  $labels.pod  }} has been trying to start for longer than 15 minutes - this could indicate a configuration error."),
				},	
			Expr: intstr.FromString(fmt.Sprintf("(kube_pod_container_status_waiting_reason{reason='ContainerCreating'} * on (namespace, namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) > 0")),
			For: "15m",
			Labels: map[string]string{"severity": "critical"},	
		},
		{
			Alert: fmt.Sprintf("ClusterSchedulableMemoryLow"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts/Cluster_Schedulable_Resources_Low.asciidoc",
				"message": fmt.Sprintf("The cluster has {{   $value  }} percent of memory requested and unavailable for scheduling for longer than 15 minutes"),
			},
			Expr:   intstr.FromString(fmt.Sprintf(" ((sum(sum by(node) (sum by(pod, node) (kube_pod_container_resource_requests_memory_bytes * on(node) group_left() (sum by(node) (kube_node_labels{label_node_role_kubernetes_io_compute='true'} == 1))) * on(pod) group_left() (sum by(pod) (kube_pod_status_phase{phase='Running'}) == 1)))) / ((sum((kube_node_labels{label_node_role_kubernetes_io_compute='true'} == 1) * on(node) group_left() (sum by(node) (kube_node_status_allocatable_memory_bytes)))))) * 100 > 85")),
			For:    "15m",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("ClusterSchedulableCPULow"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts/Cluster_Schedulable_Resources_Low.asciidoc",
				"message": fmt.Sprintf("The cluster has {{  $value  }} percent of CPU cores requested and unavailable for scheduling for longer than 15 minutes"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("((sum(sum by(node) (sum by(pod, node) (kube_pod_container_resource_requests_cpu_cores * on(node) group_left() (sum by(node) (kube_node_labels{label_node_role_kubernetes_io_compute='true'} == 1))) * on(pod) group_left() (sum by(pod) (kube_pod_status_phase{phase='Running'}) == 1)))) / ((sum((kube_node_labels{label_node_role_kubernetes_io_compute='true'} == 1) * on(node) group_left() (sum by(node) (kube_node_status_allocatable_cpu_cores)))))) * 100 > 85")),
			For:    "15m",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("PVCStorageAvailable"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts/Cluster_Schedulable_Resources_Low.asciidoc",
				"message": fmt.Sprintf("The {{  $labels.persistentvolumeclaim  }} PVC has has been {{  $value  }} percent full for longer than 15 minutes"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("((sum by(persistentvolumeclaim, namespace) (kubelet_volume_stats_used_bytes) * on ( namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) / (sum by(persistentvolumeclaim, namespace) (kube_persistentvolumeclaim_resource_requests_storage_bytes) * on ( namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'})) * 100 > 85")),
			For:    "15m",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("PVCStorageMetricsAvailable"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts/Cluster_Schedulable_Resources_Low.asciidoc",
				"message": fmt.Sprintf("PVC storage metrics are not available"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("absent(kubelet_volume_stats_available_bytes) == 1 or absent(kubelet_volume_stats_capacity_bytes) == 1 or absent(kubelet_volume_stats_used_bytes) == 1 or absent(kube_persistentvolumeclaim_resource_requests_storage_bytes) == 1")),
			For:    "15m",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("PVCStorageWillFillIn4Days"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/2.x/alerts/pvc_storage.asciidoc#pvcstoragewillfillin4hours",
				"message": fmt.Sprintf("The {{  $labels.persistentvolumeclaim  }} PVC will run of disk space in the next 4 days"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("(predict_linear(kubelet_volume_stats_available_bytes{job='kubelet'}[6h], 4 * 24 * 3600) <= 0 AND kubelet_volume_stats_available_bytes{job='kubelet'} / kubelet_volume_stats_capacity_bytes{job='kubelet'} < 0.25 )* on(namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}")),
			For:    "15m",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Alert: fmt.Sprintf("PVCStorageWillFillIn4Hours"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/2.x/alerts/pvc_storage.asciidoc#pvcstoragewillfillin4hours",
				"message": fmt.Sprintf("The {{  $labels.persistentvolumeclaim  }} PVC will run of disk space in the next 4 hours"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("( predict_linear(kubelet_volume_stats_available_bytes{job='kubelet'}[1h], 4 * 3600) <= 0 AND kubelet_volume_stats_available_bytes{job='kubelet'} / kubelet_volume_stats_capacity_bytes{job='kubelet'} < 0.25)* on(namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}")),
			For:    "15m",
			Labels: map[string]string{"severity": "critical"},
		},

		{
			Alert: fmt.Sprintf("PersistentVolumeErrors"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/2.x/alerts/pvc_storage.asciidoc#persistentvolumeerrors",
				"message": fmt.Sprintf("The PVC {{  $labels.persistentvolumeclaim  }} is in status {{  $labels.phase  }} in namespace {{  $labels.namespace }} "),
				},	
			Expr: intstr.FromString(fmt.Sprintf("(sum by(persistentvolumeclaim, namespace, phase) (kube_persistentvolumeclaim_status_phase{phase=~'Failed|Pending|Lost'}) * on ( namespace) group_left(label_monitoring_key) kube_namespace_labels{label_monitoring_key='middleware'}) > 0")),
			For: "15m",
			Labels: map[string]string{"severity": "critical"},	
		
		}}

		_, err := controllerutil.CreateOrUpdate(ctx, serverClient, rule, func() error {
		rule.ObjectMeta.Labels = map[string]string{"integreatly": "yes", monitoringConfig.GetLabelSelectorKey(): monitoringConfig.GetLabelSelector()}
		rule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:  "general.rules",
					Rules: rules,
				},
			},
		}
		return nil
	})

	if err != nil {
		logrus.Infof("Phase: %s reconcilePrometheusAlerts", err)

		return integreatlyv1alpha1.PhaseFailed, fmt.Errorf("error creating backup PrometheusRule: %w", err)
	}
	return integreatlyv1alpha1.PhaseCompleted, nil
}
func (r *Reconciler) reconcileKubeStateMetricsMonitoringAlerts(ctx context.Context, serverClient k8sclient.Client) (integreatlyv1alpha1.StatusPhase, error) {
	monitoringConfig := config.NewMonitoring(config.ProductConfig{})
	rule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ksm-monitoring-alerts",
			Namespace: r.Config.GetOperatorNamespace(),
		},
	}

	rules := []monitoringv1.Rule{
		{
			Alert: fmt.Sprintf("MiddlewareMonitoringPodCount"),
			Annotations: map[string]string{
				"sop_url": "https://github.com/RHCloudServices/integreatly-help/blob/master/sops/alerts_and_troubleshooting.md",
				"message": fmt.Sprintf("Pod count for namespace {{ $labels.namespace}} is {{  $value }}. Expected exactly 7 pods"),
			},
			Expr:   intstr.FromString(fmt.Sprintf("(1 - absent(kube_pod_status_ready{condition='true',namespace='redhat-rhmi-middleware-monitoring-operator'})) or sum(kube_pod_status_ready{condition='true',namespace='redhat-rhmi-middleware-monitoring-operator'}) != 7")),
			For:    "5m",
			Labels: map[string]string{"severity": "critical"},
		}}
	_, err := controllerutil.CreateOrUpdate(ctx, serverClient, rule, func() error {
		rule.ObjectMeta.Labels = map[string]string{"integreatly": "yes", monitoringConfig.GetLabelSelectorKey(): monitoringConfig.GetLabelSelector()}
		rule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:  "general.rules",
					Rules: rules,
				},
			},
		}

		return nil

	})
	if err != nil {
		logrus.Infof("Phase: %s reconcilePrometheusAlerts", err)

		return integreatlyv1alpha1.PhaseFailed, fmt.Errorf("error creating backup PrometheusRule: %w", err)

	}

	return integreatlyv1alpha1.PhaseCompleted, nil
}
