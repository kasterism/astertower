package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	astertowerclientset "github.com/kasterism/astertower/pkg/clients/clientset/astertower"
	"github.com/kasterism/astertower/pkg/clients/informer/externalversions"
	"github.com/kasterism/astertower/pkg/controllers"
	"github.com/kasterism/astertower/pkg/signals"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

var (
	// controller
	kubeconfig string
	mode       string

	// server
	namespace string
	listen    string
)

func init() {
	flag.StringVar(&mode, "mode", "external", "The running location of the astertower controller.")

	flag.StringVar(&namespace, "namespace", "default", "Specify a namespace for the workflow to run")
	flag.StringVar(&listen, "listen", "0.0.0.0:8080", "Specify the listening ip address and port")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	config := GetConfigOrDie()

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
		astroInformerFactory.Astertower().V1alpha1().Astros(), mode)

	go kubeInformerFactory.Start(ctx.Done())
	go astroInformerFactory.Start(ctx.Done())

	// Launch server
	// go server.Start(ctx, listen)

	if err = astroController.Run(ctx, 2); err != nil {
		klog.Fatalln("Error running controller:", err.Error())
	}
}

func GetConfigOrDie() *rest.Config {
	cfg, err := GetConfig("")
	if err != nil {
		klog.Errorln(err, "unable to get kubeconfig")
		os.Exit(1)
	}
	return cfg
}

func GetConfig(context string) (*rest.Config, error) {
	if len(kubeconfig) > 0 {
		return loadConfigWithContext("", &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}, context)
	}

	kubeconfigPath := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	if len(kubeconfigPath) == 0 {
		if c, err := rest.InClusterConfig(); err == nil {
			return c, nil
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if _, ok := os.LookupEnv("HOME"); !ok {
		u, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("could not get current user: %w", err)
		}
		loadingRules.Precedence = append(loadingRules.Precedence, filepath.Join(u.HomeDir, clientcmd.RecommendedHomeDir, clientcmd.RecommendedFileName))
	}

	return loadConfigWithContext("", loadingRules, context)
}

func loadConfigWithContext(apiServerURL string, loader clientcmd.ClientConfigLoader, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader,
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: apiServerURL,
			},
			CurrentContext: context,
		}).ClientConfig()
}
