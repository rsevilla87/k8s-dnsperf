package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

func NewInfra(uuid string) (Infra, error) {
	clientSet, err := newClientSet()
	if err != nil {
		return Infra{}, err
	}
	return Infra{
		ClientSet: clientSet,
		UUID:      uuid,
	}, nil
}

func newClientSet() (*kubernetes.Clientset, error) {
	var kubeconfig string
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); kubeconfig == "" && !os.IsNotExist(err) {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfigOrDie(restConfig), nil
}

func (i *Infra) Deploy() error {
	fmt.Printf("Creating required infrastructure")
	_, err := i.ClientSet.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Namespace: %w", err)
	}
	_, err = i.ClientSet.AppsV1().DaemonSets(namespace.Name).Create(context.TODO(), &dnsPerfDS, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create DaemonSet: %w", err)
	}
	if err := waitForDS(i.ClientSet); err != nil {
		return err
	}
	return nil
}

func (i *Infra) Destroy() error {
	fmt.Printf("Destroying required infrastructure")
	err := i.ClientSet.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
	return err
}

func waitForDS(clientSet *kubernetes.Clientset) error {
	fmt.Printf("Waiting for DaemonSet %s/%s pods to be running\n", namespace.Name, dnsPerfDS.Name)
	watcher, err := clientSet.AppsV1().DaemonSets(namespace.Name).Watch(context.TODO(), metav1.ListOptions{TimeoutSeconds: ptr.To[int64](60)})
	if err != nil {
		return err
	}
	for event := range watcher.ResultChan() {
		ds := event.Object.(*appsv1.DaemonSet)
		if event.Type == watch.Modified {
			if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
				watcher.Stop()
				return nil
			}
		}
	}
	return nil
}
