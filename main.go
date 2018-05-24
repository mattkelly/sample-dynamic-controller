package main

import (
	"flag"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building dynamic clientset: %s", err.Error())
	}

	resource := schema.GroupVersionResource{
		Group:    "samplecontroller.k8s.io",
		Version:  "v1alpha1",
		Resource: "foos",
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	dynamicInformer := cache.NewSharedIndexInformer(
		listWatcher(dynamicClient, resource),
		nil,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	controller := NewController(kubeClient, dynamicClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		dynamicInformer)

	go kubeInformerFactory.Start(stopCh)
	go dynamicInformer.Run(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func listWatcher(client dynamic.Interface, resource schema.GroupVersionResource) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			// We want to list this resource in all namespaces if it's namespace scoped, so not passing namespace is ok.
			return client.Resource(resource).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			// We want to list this resource in all namespaces if it's namespace scoped, so not passing namespace is ok.
			return client.Resource(resource).Watch(options)
		},
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
