package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

// Creates a new infra instance
func NewInfra(uuid, selector string, records int, recordType RecordType) (*Infra, error) {
	clientSet, restConfig, err := newClientSet()
	if err != nil {
		return &Infra{}, err
	}
	return &Infra{
		ClientSet:  clientSet,
		RestConfig: restConfig,
		UUID:       uuid,
		Selector:   selector,
		Records:    records,
		RecordType: recordType,
	}, nil
}

func newClientSet() (*kubernetes.Clientset, *rest.Config, error) {
	var kubeconfig string
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); kubeconfig == "" && !os.IsNotExist(err) {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	restConfig.QPS = QPS
	restConfig.Burst = Burst
	if err != nil {
		return nil, restConfig, err
	}
	return kubernetes.NewForConfigOrDie(restConfig), restConfig, nil
}

// Deploy k8s-dnsperf assets
func (i *Infra) Deploy() error {
	limiter := rate.NewLimiter(QPS, Burst)
	log.Info().Msg("Creating benchmark assets 🚧")
	nodeSelector, err := labels.ConvertSelectorToLabelsMap(i.Selector)
	if err != nil {
		return err
	}
	dnsPerfDS.Spec.Template.Spec.NodeSelector = nodeSelector
	log.Debug().Msgf("Creating namespace: %s", namespace.Name)
	_, err = i.ClientSet.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Namespace: %w", err)
	}
	log.Debug().Msgf("Creating %d services", i.Records)
	for j := 1; j < i.Records; j++ {
		service.Name = fmt.Sprintf("%s-%d", K8sDNSPerf, j)
		go func(svc corev1.Service) {
			limiter.Wait(context.TODO())
			_, err := i.ClientSet.CoreV1().Services(namespace.Name).Create(context.TODO(), &svc, metav1.CreateOptions{})
			if err != nil {
				log.Fatal().Msgf("failed to create Service: %v", err)
			}
		}(service)
	}
	i.Services, err = i.ClientSet.CoreV1().Services(K8sDNSPerf).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + K8sDNSPerf,
	})
	if err != nil {
		return err
	}
	recordsCM.Data["records"] = i.genRecords()
	log.Debug().Msgf("Creating ConfigMap: %s", recordsCM.Name)
	_, err = i.ClientSet.CoreV1().ConfigMaps(K8sDNSPerf).Create(context.TODO(), &recordsCM, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	log.Debug().Msgf("Creating DaemonSet: %s", dnsPerfDS.Name)
	_, err = i.ClientSet.AppsV1().DaemonSets(namespace.Name).Create(context.TODO(), &dnsPerfDS, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create DaemonSet: %w", err)
	}
	i.ClientPods, err = waitForDS(i.ClientSet)
	return err
}

// Destroy k8s-dnsperf assets
func (i *Infra) Destroy() error {
	log.Info().Msg("Destroying benchmark assets 💥")
	err := i.ClientSet.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
	return err
}

func waitForDS(clientSet *kubernetes.Clientset) (*corev1.PodList, error) {
	var podList *corev1.PodList
	log.Info().Msgf("Waiting for DaemonSet %s/%s pods to be running", namespace.Name, dnsPerfDS.Name)
	ds, err := clientSet.AppsV1().DaemonSets(namespace.Name).Get(context.TODO(), dnsPerfDS.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return nil, err
	}
	// TODO handle timeout
	watcher, err := clientSet.AppsV1().DaemonSets(namespace.Name).Watch(context.TODO(), metav1.ListOptions{TimeoutSeconds: ptr.To[int64](60)})
	if err != nil {
		return nil, err
	}
	for event := range watcher.ResultChan() {
		ds := event.Object.(*appsv1.DaemonSet)
		if event.Type == watch.Modified {
			if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
				watcher.Stop()
				break
			}
		}
	}
	podList, err = clientSet.CoreV1().Pods(namespace.Name).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + K8sDNSPerf,
	})
	return podList, err
}

func (i *Infra) genRecords() string {
	var records string
	records += fmt.Sprintf("kubernetes.default.svc.cluster.local %s\n", i.RecordType)
	for _, svc := range i.Services.Items {
		records += fmt.Sprintf("%s.%s.svc.cluster.local %s\n", svc.Name, svc.Namespace, i.RecordType)
	}
	return records
}
