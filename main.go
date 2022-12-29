package main

import (
	"flag"
	"time"

	astertowerclientset "github.com/kasterism/astertower/pkg/clients/clientset/astertower"
	"github.com/kasterism/astertower/pkg/clients/informer/externalversions"
	"github.com/kasterism/astertower/pkg/controllers"
	"github.com/kasterism/astertower/pkg/signals"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	masterURL  string
	kubeconfig string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", clientcmd.RecommendedHomeFile, "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalln(err)
	}

	clientSet, err := astertowerclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalln(err)
	}

	factory := externalversions.NewSharedInformerFactory(clientSet, time.Second*30)

	astroController := controllers.NewAstroController(factory.Astertower().V1alpha1().Astros())

	go factory.Start(stopCh)

	if err = astroController.Run(3, stopCh); err != nil {
		klog.Fatalln("Error running controller:", err.Error())
	}
}
