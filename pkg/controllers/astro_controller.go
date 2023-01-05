package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/kasterism/astertower/pkg/apis/v1alpha1"
	astertowerclientset "github.com/kasterism/astertower/pkg/clients/clientset/astertower"
	"github.com/kasterism/astertower/pkg/clients/clientset/astertower/scheme"
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
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
	// maxRetries is the number of times an astro will be retried before it is dropped out of the queue.
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
	kubeClientset  kubernetes.Interface
	astroClientset astertowerclientset.Interface

	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder

	syncHandler  func(ctx context.Context, key string) error
	enqueueAstro func(astro *v1alpha1.Astro)

	astroLister      astrolister.AstroLister
	deploymentLister appslisters.DeploymentLister
	serviceLister    corelisters.ServiceLister

	astroListerSynced      cache.InformerSynced
	deploymentListerSynced cache.InformerSynced
	serviceListerSynced    cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewAstroController(kubeClientset kubernetes.Interface, astroClientset astertowerclientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer, serviceInformer coreinformers.ServiceInformer,
	astroInformer informers.AstroInformer) *AstroController {

	eventBroadcaster := record.NewBroadcaster()

	astroController := &AstroController{
		kubeClientset:    kubeClientset,
		astroClientset:   astroClientset,
		eventBroadcaster: eventBroadcaster,
		eventRecorder:    eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "astro-controller"}),
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "astro"),
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
		AddFunc: astroController.handleObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)
			if oldDeployment.ResourceVersion == newDeployment.ResourceVersion {
				return
			}
			astroController.handleObject(newDeployment)
		},
		DeleteFunc: astroController.handleObject,
	})
	if err != nil {
		klog.Fatalln("Failed to add deployment event handlers")
	}

	_, err = serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: astroController.handleObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldService := oldObj.(*corev1.Service)
			newService := newObj.(*corev1.Service)
			if oldService.ResourceVersion == newService.ResourceVersion {
				return
			}
			astroController.handleObject(newService)
		},
		DeleteFunc: astroController.handleObject,
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

func (c *AstroController) Run(ctx context.Context, worker int) error {
	defer runtime.HandleCrash()

	// Start events processing pipeline.
	c.eventBroadcaster.StartStructuredLogging(0)
	c.eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: c.kubeClientset.CoreV1().Events("")})
	defer c.eventBroadcaster.Shutdown()

	defer c.queue.ShuttingDown()

	klog.InfoS("Starting controller", "controller", "astro")
	defer klog.InfoS("Shutting down controller", "controller", "astro")

	if !cache.WaitForNamedCacheSync("astro", ctx.Done(), c.astroListerSynced, c.deploymentListerSynced, c.serviceListerSynced) {
		return fmt.Errorf("failed to wati for caches to sync")
	}

	for i := 0; i < worker; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
	return nil
}

func (c *AstroController) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *AstroController) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(ctx, key.(string))
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

func (c *AstroController) syncAstro(ctx context.Context, key string) error {
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
		return c.syncDelete(ctx, astro)
	}

	for _, finalizer := range astro.Finalizers {
		if finalizer == AstroFinalizer {
			return c.syncUpdate(ctx, astro)
		}
	}

	// TODO: do something
	return c.syncCreate(ctx, astro)
}

func (c *AstroController) syncCreate(ctx context.Context, astro *v1alpha1.Astro) error {
	klog.Infof("Sync create astro: %s\n", astro.Name)

	// Add finalizer when creating resources
	astro.Finalizers = append(astro.Finalizers, AstroFinalizer)

	for _, star := range astro.Spec.Stars {
		err := c.newDeployment(ctx, astro, &star)
		if err != nil {
			return err
		}

		err = c.newService(ctx, astro, &star)
		if err != nil {
			return err
		}
	}

	err := c.newAstermule(ctx, astro)
	if err != nil {
		return err
	}

	statusCopy := astro.Status.DeepCopy()

	astro, err = c.astroClientset.
		AstertowerV1alpha1().
		Astros(astro.Namespace).
		Update(ctx, astro, metav1.UpdateOptions{})
	if err != nil {
		runtime.HandleError(err)
		return err
	}

	statusCopy.Conditions = append(statusCopy.Conditions, v1alpha1.AstroCondition{
		Type:               "Ready",
		Status:             v1alpha1.AstroConditionInitialized,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})
	statusCopy.Phase = v1alpha1.AstroConditionInitialized
	statusCopy.NodeNumber = int32(len(statusCopy.DeploymentRef))
	statusCopy.ReadyNodeNumber = 0

	return c.syncStatus(ctx, astro, *statusCopy)
}

func (c *AstroController) syncUpdate(ctx context.Context, astro *v1alpha1.Astro) error {
	klog.Infof("Sync update astro: %s\n", astro.Name)

	newStatus := astro.Status.DeepCopy()
	var readyNode int32 = 0

	if astro.Status.Phase == v1alpha1.AstroConditionInitialized {
		// Check the workflow engine startup status
		astermuleRef := astro.Status.AstermuleRef
		pod, err := c.kubeClientset.
			CoreV1().
			Pods(astermuleRef.Namespace).
			Get(ctx, astermuleRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to get astermule through astermuleRef", "astermule", klog.KRef(pod.Namespace, pod.Name))
			return err
		}
		if pod.Status.Phase == corev1.PodRunning {
			newStatus.WorkflowEngineInitialized = true
		}

		// Count the number of successfully deployed nodes
		for _, deployRef := range astro.Status.DeploymentRef {
			deployment, err := c.kubeClientset.
				AppsV1().
				Deployments(deployRef.Namespace).
				Get(ctx, deployRef.Name, metav1.GetOptions{})
			if err != nil {
				klog.ErrorS(err, "Failed to get deployment through deploymentRef", "deployment", klog.KRef(deployRef.Namespace, deployRef.Name))
				return err
			}
			if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
				readyNode++
			}
		}
		newStatus.ReadyNodeNumber = readyNode

		// Check whether the Ready state is satisfied, and update if it is
		if newStatus.ReadyNodeNumber == newStatus.NodeNumber && newStatus.WorkflowEngineInitialized {
			newStatus.Conditions = append(newStatus.Conditions, v1alpha1.AstroCondition{
				Type:               "Ready",
				Status:             v1alpha1.AstroConditionReady,
				LastTransitionTime: metav1.NewTime(time.Now()),
			})
			newStatus.Phase = v1alpha1.AstroConditionReady
		}
	}

	return c.syncStatus(ctx, astro, *newStatus)
}

func (c *AstroController) syncDelete(ctx context.Context, astro *v1alpha1.Astro) error {
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
		Update(ctx, astro, metav1.UpdateOptions{})
	if err != nil {
		runtime.HandleError(err)
		return err
	}

	return nil
}

func (c *AstroController) syncStatus(ctx context.Context, astro *v1alpha1.Astro, newStatus v1alpha1.AstroStatus) error {
	if reflect.DeepEqual(astro.Status, newStatus) {
		return nil
	}

	astro.Status = newStatus
	_, err := c.astroClientset.
		AstertowerV1alpha1().
		Astros(astro.Namespace).
		UpdateStatus(ctx, astro, metav1.UpdateOptions{})

	return err
}

func (c *AstroController) newDeployment(ctx context.Context, astro *v1alpha1.Astro, star *v1alpha1.AstroStar) error {
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
		Create(ctx, deployment, metav1.CreateOptions{})
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

func (c *AstroController) newService(ctx context.Context, astro *v1alpha1.Astro, star *v1alpha1.AstroStar) error {
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
		Create(ctx, service, metav1.CreateOptions{})
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

func (c *AstroController) newAstermule(ctx context.Context, astro *v1alpha1.Astro) error {
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
		Create(ctx, pod, metav1.CreateOptions{})
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

func (c *AstroController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "Astro" {
			return
		}

		astro, err := c.astroLister.Astros(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of astro '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueue(astro)
		return
	}
}
