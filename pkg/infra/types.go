package infra

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

const (
	QPS   = 100
	Burst = 100
)

const K8sDNSPerf = "k8s-dnsperf"

type RecordType string

const RecordA RecordType = "A"
const RecordAAAA RecordType = "AAAA"

type Infra struct {
	UUID       string
	ClientSet  *kubernetes.Clientset
	RestConfig *rest.Config
	Selector   string
	Records    int
	RecordType RecordType
	ClientPods *corev1.PodList
	Services   *corev1.ServiceList
}

var namespace = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: K8sDNSPerf,
	},
}

var service = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: K8sDNSPerf,
		Labels: map[string]string{
			"app": K8sDNSPerf,
		},
	},
	Spec: corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Port:       80,
			TargetPort: intstr.FromInt(80),
		},
		},
	},
}

var dnsPerfDS = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{
		Name:      K8sDNSPerf,
		Namespace: K8sDNSPerf,
	},
	Spec: appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": K8sDNSPerf,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": K8sDNSPerf,
				},
			},
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: ptr.To[int64](0),
				Containers: []corev1.Container{
					{
						Name:  K8sDNSPerf,
						Image: "quay.io/cloud-bulldozer/k8s-dnsperf:latest",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "dnsperf-records",
								MountPath: "/records",
								SubPath:   "records",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "dnsperf-records",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "dnsperf-records",
								},
							},
						},
					},
				},
			},
		},
	},
}

var recordsCM = corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "dnsperf-records",
		Namespace: K8sDNSPerf,
	},
	Data: map[string]string{
		"records": "",
	},
}
