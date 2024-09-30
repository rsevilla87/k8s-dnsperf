package infra

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const dnsperf = "dnsperf"

type Infra struct {
	UUID      string
	ClientSet *kubernetes.Clientset
}

var namespace = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: dnsperf,
	},
}

var dnsPerfDS = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{
		Name:      dnsperf,
		Namespace: dnsperf,
	},
	Spec: appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": dnsperf,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": dnsperf,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  dnsperf,
						Image: "quay.io/cloud-bulldozer/dnsperf:latest",
					},
				},
			},
		},
	},
}
