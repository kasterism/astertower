package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/kasterism/astertower/pkg/apis/v1alpha1"
	astertowerclientset "github.com/kasterism/astertower/pkg/clients/clientset/astertower"
	informers "github.com/kasterism/astertower/pkg/clients/informer/externalversions/apis/v1alpha1"
	astrolister "github.com/kasterism/astertower/pkg/clients/lister/apis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// name of finalizer
	AstroFinalizer = "astros.astertower.kasterism.io"
	AstermuleImage = "kasterism/astermule:v0.1.0-rc"
	// maxRetries is the number of times a deployment will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a deployment is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

var (
	astroControllerKind       = v1alpha1.SchemeGroupVersion.WithKind("Astro")
	replicas            int32 = 1
)

type AstroController struct {
	kubeClientset kubernetes.Interface

	astroClientset astertowerclientset.Interface

	recorder record.EventRecorder

	syncHandler func(key string) error

	enqueueAstro func(astro *v1alpha1.Astro)

	astroLister astrolister.AstroLister

	deploymentLister appslisters.DeploymentLister

	serviceLister corelisters.ServiceLister

	astroListerSynced cache.InformerSynced

	deploymentListerSynced cache.InformerSynced

	serviceListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewAstroController(kubeClientset kubernetes.Interface, astroClientset astertowerclientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer, serviceInformer coreinformers.ServiceInformer,
	astroInformer informers.AstroInformer) *AstroController {
	astroController := &AstroController{
		kubeClientset:  kubeClientset,
		astroClientset: astroClientset,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "astro"),
	}

	_, err := astroInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    astroController.addAstro,
		DeleteFunc: astroController.deleteAstro,
		UpdateFunc: astroController.updateAstro,
	})
	if err != nil {
		klog.Fatalln("Failed to add astro event handlers")
	}

	_, err = deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: astroController.handleDeployment,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)
			if oldDeployment.ResourceVersion == newDeployment.ResourceVersion {
				return
			}
			astroController.handleDeployment(newDeployment)
		},
		DeleteFunc: astroController.handleDeployment,
	})
	if err != nil {
		klog.Fatalln("Failed to add deployment event handlers")
	}

	_, err = serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: astroController.handleService,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldService := oldObj.(*corev1.Service)
			newService := newObj.(*corev1.Service)
			if oldService.ResourceVersion == newService.ResourceVersion {
				return
			}
			astroController.handleService(newService)
		},
		DeleteFunc: astroController.handleService,
	})
	if err != nil {
		klog.Fatalln("Failed to add service event handlers")
	}

	astroController.syncHandler = astroController.syncAstro
	astroController.enqueueAstro = astroController.enqueue

	astroController.astroLister = astroInformer.Lister()
	astroController.deploymentLister = deploymentInformer.Lister()
	astroController.serviceLister = serviceInformer.Lister()
	astroController.astroListerSynced = astroInformer.Informer().HasSynced
	astroController.deploymentListerSynced = deploymentInformer.Informer().HasSynced
	astroController.serviceListerSynced = serviceInformer.Informer().HasSynced

	return astroController
}

func (c *AstroController) Run(thread int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	// TODO: Start events broadcaster

	defer c.queue.ShuttingDown()

	klog.InfoS("Starting controller", "controller", "astro")
	defer klog.InfoS("Shutting down controller", "controller", "astro")

	if !cache.WaitForNamedCacheSync("astro", stopCh, c.astroListerSynced, c.deploymentListerSynced, c.serviceListerSynced) {
		return fmt.Errorf("failed to wati for caches to sync")
	}

	for i := 0; i < thread; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

func (c *AstroController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *AstroController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *AstroController) handleErr(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
		c.queue.Forget(key)
		return
	}

	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if c.queue.NumRequeues(key) < maxRetries {
		klog.V(2).InfoS("Error syncing astro", "astro", klog.KRef(ns, name), "err", err)
		c.queue.AddRateLimited(key)
		return
	}

	runtime.HandleError(err)
	klog.V(2).InfoS("Dropping astro out of the queue", "astro", klog.KRef(ns, name), "err", err)
	c.queue.Forget(key)
}

func (c *AstroController) addAstro(obj interface{}) {
	item := obj.(*v1alpha1.Astro)
	klog.V(4).InfoS("Adding astro", "astro", klog.KObj(item))
	c.enqueueAstro(item)
}

func (c *AstroController) updateAstro(old, new interface{}) {
	oldItem := old.(*v1alpha1.Astro)
	newItem := new.(*v1alpha1.Astro)
	if oldItem.ResourceVersion == newItem.ResourceVersion {
		return
	}

	klog.V(4).InfoS("Updating astro", "astro", klog.KObj(oldItem))

	c.enqueueAstro(newItem)
}

func (c *AstroController) deleteAstro(obj interface{}) {
	item, ok := obj.(*v1alpha1.Astro)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		item, ok = tombstone.Obj.(*v1alpha1.Astro)
		if !ok {
			runtime.HandleError(fmt.Errorf("tombstone contained object that is not a Astro %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Deleting astro", "astro", klog.KObj(item))

	c.enqueueAstro(item)
}

func (c *AstroController) enqueue(astro *v1alpha1.Astro) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(astro)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *AstroController) syncAstro(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing astro", "astro", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing astro", "astro", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	astro, err := c.astroLister.Astros(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("Astro has been deleted", "astro", klog.KRef(namespace, name))
		return nil
	}
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to get astro by: %s/%s", namespace, name))
		return err
	}
	if !astro.DeletionTimestamp.IsZero() {
		return c.syncDelete(astro)
	}

	for _, finalizer := range astro.Finalizers {
		if finalizer == AstroFinalizer {
			return c.syncUpdate(astro)
		}
	}

	// TODO: do something
	return c.syncCreate(astro)
}

func (c *AstroController) syncCreate(astro *v1alpha1.Astro) error {
	klog.Infof("Sync create astro: %s\n", astro.Name)

	// Add finalizer when creating resources
	astro.Finalizers = append(astro.Finalizers, AstroFinalizer)

	for _, star := range astro.Spec.Stars {
		err := c.newDeployment(astro, &star)
		if err != nil {
			return err
		}

		err = c.newService(astro, &star)
		if err != nil {
			return err
		}
	}

	err := c.newAstermule(astro)
	if err != nil {
		return err
	}

	statusCopy := astro.Status.DeepCopy()

	astro, err = c.astroClientset.
		AstertowerV1alpha1().
		Astros(astro.Namespace).
		Update(context.TODO(), astro, metav1.UpdateOptions{})
	if err != nil {
		runtime.HandleError(err)
		return err
	}

	statusCopy.Initialized = true
	statusCopy.Conditions = append(statusCopy.Conditions, v1alpha1.AstroCondition{
		Type:               "Ready",
		Status:             v1alpha1.AstroConditionInitialized,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

	fmt.Println(statusCopy.AstermuleRef)

	return c.syncStatus(astro, *statusCopy)
}

func (c *AstroController) syncUpdate(astro *v1alpha1.Astro) error {
	klog.Infof("Sync update astro: %s\n", astro.Name)

	return c.syncStatus(astro, astro.Status)
}

func (c *AstroController) syncDelete(astro *v1alpha1.Astro) error {
	klog.Infof("Sync delete astro: %s\n", astro.Name)

	// Remove finalizer when deleting resources
	for i, finalizer := range astro.Finalizers {
		if finalizer == AstroFinalizer {
			astro.Finalizers[i] = astro.Finalizers[len(astro.Finalizers)-1]
			astro.Finalizers = astro.Finalizers[:len(astro.Finalizers)-1]
		}
	}

	_, err := c.astroClientset.AstertowerV1alpha1().
		Astros(astro.Namespace).
		Update(context.TODO(), astro, metav1.UpdateOptions{})
	if err != nil {
		runtime.HandleError(err)
		return err
	}

	return nil
}

func (c *AstroController) syncStatus(astro *v1alpha1.Astro, newStatus v1alpha1.AstroStatus) error {
	if reflect.DeepEqual(astro.Status, newStatus) {
		return nil
	}

	astro.Status = newStatus
	_, err := c.astroClientset.
		AstertowerV1alpha1().
		Astros(astro.Namespace).
		UpdateStatus(context.TODO(), astro, metav1.UpdateOptions{})

	return err
}

func (c *AstroController) newDeployment(astro *v1alpha1.Astro, star *v1alpha1.AstroStar) error {
	labels := map[string]string{
		"star": star.Name,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: astro.Namespace,
			Name:      star.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(astro, astroControllerKind),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  star.Name,
							Image: star.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: star.Port,
									HostPort:      star.Port,
								},
							},
						},
					},
				},
			},
		},
	}

	deployment, err := c.kubeClientset.
		AppsV1().
		Deployments(astro.Namespace).
		Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		klog.Errorln("Failed to create deployment:", err)
		return err
	}

	astro.Status.DeploymentRef = append(astro.Status.DeploymentRef, v1alpha1.AstroRef{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	})

	return nil
}

func (c *AstroController) newService(astro *v1alpha1.Astro, star *v1alpha1.AstroStar) error {
	labels := map[string]string{
		"star": star.Name,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: astro.Namespace,
			Name:      star.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(astro, astroControllerKind),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       star.Port,
					TargetPort: intstr.FromInt(int(star.Port)),
				},
			},
			Selector: labels,
		},
	}

	service, err := c.kubeClientset.
		CoreV1().
		Services(astro.Namespace).
		Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		klog.Errorln("Failed to create service:", err)
		return err
	}

	astro.Status.ServiceRef = append(astro.Status.ServiceRef, v1alpha1.AstroRef{
		Name:      service.Name,
		Namespace: service.Namespace,
	})

	return nil
}

func (c *AstroController) newAstermule(astro *v1alpha1.Astro) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: astro.Namespace,
			Name:      astro.Name + "-astermule",
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(astro, astroControllerKind),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  astro.Name + "-astermule",
					Image: AstermuleImage,
					// TODO: Complete command of astermule
				},
			},
		},
	}

	pod, err := c.kubeClientset.
		CoreV1().
		Pods(astro.Namespace).
		Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		klog.Errorln("Failed to create astermule:", err)
		return err
	}

	astro.Status.AstermuleRef = v1alpha1.AstroRef{
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}

	return nil
}

func (c *AstroController) handleDeployment(item interface{}) {

}

func (c *AstroController) handleService(item interface{}) {

}
