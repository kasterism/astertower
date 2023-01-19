package main

import (
	"flag"
	"time"

	astertowerclientset "github.com/kasterism/astertower/pkg/clients/clientset/astertower"
	"github.com/kasterism/astertower/pkg/clients/informer/externalversions"
	"github.com/kasterism/astertower/pkg/controllers"
	"github.com/kasterism/astertower/pkg/server"
	"github.com/kasterism/astertower/pkg/signals"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	// controller
	masterURL  string
	kubeconfig string

	// server
	namespace string
	listen    string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", clientcmd.RecommendedHomeFile, "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	flag.StringVar(&namespace, "namespace", "default", "Specify a namespace for the workflow to run")
	flag.StringVar(&listen, "listen", "0.0.0.0:8080", "Specify the listening ip address and port")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalln(err)
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalln(err)
	}

	astroClientset, err := astertowerclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalln(err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClientset, time.Second*30)
	astroInformerFactory := externalversions.NewSharedInformerFactory(astroClientset, time.Second*30)

	astroController := controllers.NewAstroController(kubeClientset, astroClientset,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Core().V1().Services(),
		astroInformerFactory.Astertower().V1alpha1().Astros())

	go kubeInformerFactory.Start(ctx.Done())
	go astroInformerFactory.Start(ctx.Done())

	// Launch server
	go server.Start(ctx, listen)

	if err = astroController.Run(ctx, 2); err != nil {
		klog.Fatalln("Error running controller:", err.Error())
	}
}
