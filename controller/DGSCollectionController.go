package controller

import (
	"fmt"
	"time"

	"github.com/dgkanatsios/azuregameserversscalingkubernetes/shared"

	typeddgsv1alpha1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/clientset/versioned/typed/azuregaming/v1alpha1"

	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	cache "k8s.io/client-go/tools/cache"

	record "k8s.io/client-go/tools/record"
	workqueue "k8s.io/client-go/util/workqueue"

	dgsclientset "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/clientset/versioned"
	kubernetes "k8s.io/client-go/kubernetes"

	informerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/informers/externalversions/azuregaming/v1alpha1"
	listerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/listers/azuregaming/v1alpha1"

	dgsscheme "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/clientset/versioned/scheme"
	log "github.com/sirupsen/logrus"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	dgsv1alpha1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/apis/azuregaming/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	dgsColControllerAgentName = "dedigated-game-server-collection-controller"
)

type DedicatedGameServerCollectionController struct {
	dgsColClient       typeddgsv1alpha1.DedicatedGameServerCollectionsGetter
	dgsColLister       listerdgs.DedicatedGameServerCollectionLister
	dgsColListerSynced cache.InformerSynced

	dgsClient       typeddgsv1alpha1.DedicatedGameServersGetter
	dgsLister       listerdgs.DedicatedGameServerLister
	dgsListerSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewDedicatedGameServerCollectionController(client *kubernetes.Clientset, dgsclient *dgsclientset.Clientset,
	dgsColInformer informerdgs.DedicatedGameServerCollectionInformer, dgsInformer informerdgs.DedicatedGameServerInformer) *DedicatedGameServerCollectionController {
	dgsscheme.AddToScheme(dgsscheme.Scheme)
	log.Info("Creating Event broadcaster for DedicatedGameServerCollection controller")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(dgsscheme.Scheme, corev1.EventSource{Component: dgsColControllerAgentName})

	c := &DedicatedGameServerCollectionController{
		dgsColClient:       dgsclient.AzuregamingV1alpha1(),
		dgsColLister:       dgsColInformer.Lister(),
		dgsColListerSynced: dgsColInformer.Informer().HasSynced,
		dgsClient:          dgsclient.AzuregamingV1alpha1(),
		dgsLister:          dgsInformer.Lister(),
		dgsListerSynced:    dgsInformer.Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DedicatedGameServerCollectionSync"),
		recorder:           recorder,
	}

	log.Info("Setting up event handlers for DedicatedGameServerCollection controller")

	dgsColInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Print("DedicatedGameServerCollection controller - add DGSCol")
				c.handleDedicatedGameServerCollection(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Print("DedicatedGameServerCollection controller - update DGSCol")
				oldDGSCol := oldObj.(*dgsv1alpha1.DedicatedGameServerCollection)
				newDGSCol := newObj.(*dgsv1alpha1.DedicatedGameServerCollection)

				if oldDGSCol.ResourceVersion == newDGSCol.ResourceVersion {
					return
				}

				//only enqueue if if the number of requested replicas has changed
				//TODO: should we allow other updates?
				if oldDGSCol.Spec.Replicas != newDGSCol.Spec.Replicas {
					c.handleDedicatedGameServerCollection(newObj)
				}

			},
			DeleteFunc: func(obj interface{}) {
				log.Print("DedicatedGameServerCollection controller - delete DGSCol")
			},
		},
	)

	dgsInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Print("DedicatedGameServerCollection controller - add DGS")
				c.handleDedicatedGameServer(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Print("DedicatedGameServerCollection controller - update DGS")
				oldDGS := oldObj.(*dgsv1alpha1.DedicatedGameServer)
				newDGS := newObj.(*dgsv1alpha1.DedicatedGameServer)

				if oldDGS.ResourceVersion == newDGS.ResourceVersion {
					return
				}

				c.handleDedicatedGameServer(newObj)
			},
		},
	)

	return c
}

// processNextWorkItem deals with one key off the queue.  It returns false
// when it's time to quit.
func (c *DedicatedGameServerCollectionController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// DedicatedGameServer resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		log.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the DedicatedGameServer resource
// with the current status of the resource.
func (c *DedicatedGameServerCollectionController) syncHandler(key string) error {

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DedicatedGameServerCollection resource with this namespace/name
	dgsColTemp, err := c.dgsColLister.DedicatedGameServerCollections(namespace).Get(name)
	if err != nil {
		// The DedicatedGameServerCollection resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("dgsCol '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Find out how many DedicatedGameServer replicas exist for this DedicatedGameServerCollection

	set := labels.Set{
		shared.LabelDedicatedGameServerCollectionName: dgsColTemp.Name,
	}
	// we seach via Labels, each DGS will have the DGSCol name as a Label
	selector := labels.SelectorFromSet(set)
	dgsExisting, err := c.dgsLister.DedicatedGameServers(dgsColTemp.Namespace).List(selector)

	if err != nil {
		log.Error(err.Error())
		return err
	}

	dgsExistingCount := len(dgsExisting)

	// if there are less DedicatedGameServers than the ones we requested
	if dgsExistingCount < int(dgsColTemp.Spec.Replicas) {
		//create them
		increaseCount := int(dgsColTemp.Spec.Replicas) - dgsExistingCount
		for i := 0; i < increaseCount; i++ {
			// create a random name for the dedicated name server
			// the corresponding pod will have the same name as well
			dgsName := dgsColTemp.Name + "-" + shared.RandString(5)

			// first, get random ports
			var portsInfoExtended []dgsv1alpha1.PortInfoExtended
			for _, portInfo := range dgsColTemp.Spec.Ports {
				//get a random port
				hostport, errPort := portRegistry.GetNewPort(dgsName)
				if errPort != nil {
					return errPort
				}

				portsInfoExtended = append(portsInfoExtended, dgsv1alpha1.PortInfoExtended{
					PortInfo: portInfo,
					HostPort: int32(hostport),
				})
			}

			// each dedicated game server will have a name of
			// DedicatedGameServerCollectioName + "-" + random name
			dgs := shared.NewDedicatedGameServer(dgsColTemp, dgsName, portsInfoExtended, dgsColTemp.Spec.StartMap, dgsColTemp.Spec.Image)
			_, err := c.dgsClient.DedicatedGameServers(namespace).Create(dgs)

			if err != nil {
				log.Error(err.Error())
				return err
			}
		}

		c.recorder.Event(dgsColTemp, corev1.EventTypeNormal, shared.DedicatedGameServerReplicasChanged, fmt.Sprintf(shared.MessageReplicasIncreased, "DedicatedGameServerCollection", dgsColTemp.Name, increaseCount))

	} else if dgsExistingCount > int(dgsColTemp.Spec.Replicas) { //if there are more DGS than the ones we requested
		// we need to decrease our DGS for this collection
		// to accomplish this, we'll first find the number of DGS we need to decrease
		decreaseCount := dgsExistingCount - int(dgsColTemp.Spec.Replicas)
		// we'll remove random instances of DGS from our DGSCol
		indexesToDecrease := shared.GetRandomIndexes(dgsExistingCount, decreaseCount)

		for i := 0; i < len(indexesToDecrease); i++ {
			dgsToMarkForDeletionTemp, err := c.dgsLister.DedicatedGameServers(namespace).Get(dgsExisting[indexesToDecrease[i]].Name)

			if err != nil {
				log.Error(err.Error())
				return err
			}
			dgsToMarkForDeletionToUpdate := dgsToMarkForDeletionTemp.DeepCopy()
			// update the DGS so it has no owners
			dgsToMarkForDeletionToUpdate.ObjectMeta.OwnerReferences = nil
			//remove the DGSCol name from the DGS labels
			delete(dgsToMarkForDeletionToUpdate.ObjectMeta.Labels, shared.LabelDedicatedGameServerCollectionName)
			//set its state as marked for deletion
			dgsToMarkForDeletionToUpdate.Status.GameServerState = shared.GameServerStateMarkedForDeletion
			dgsToMarkForDeletionToUpdate.Labels[shared.LabelGameServerState] = shared.GameServerStateMarkedForDeletion
			//update the DGS CRD
			_, err = c.dgsClient.DedicatedGameServers(namespace).Update(dgsToMarkForDeletionToUpdate)
			if err != nil {
				log.Error(err.Error())
				return err
			}

		}

		c.recorder.Event(dgsColTemp, corev1.EventTypeNormal, shared.DedicatedGameServerReplicasChanged, fmt.Sprintf(shared.MessageReplicasDecreased, "DedicatedGameServerCollection", dgsColTemp.Name, decreaseCount))
	}

	//update DGSCol
	//fetch latest version
	dgsColTemp, err = c.dgsColLister.DedicatedGameServerCollections(namespace).Get(name)
	if err != nil {
		log.Errorf("Cannot fetch DGSCol %s", name)
	}
	dgsColToUpdate := dgsColTemp.DeepCopy()

	err = c.modifyAvailableReplicas(dgsColToUpdate)
	if err != nil {
		c.recorder.Event(dgsColTemp, corev1.EventTypeWarning, "Error setting DedicatedGameServerCollection - GameServer status", err.Error())
		return err
	}

	//modify DGSCol.Status.DGSState
	err = c.assignGameServerCollectionState(dgsColToUpdate)
	if err != nil {
		c.recorder.Event(dgsColTemp, corev1.EventTypeWarning, "Error setting DedicatedGameServerCollection - GameServer status", err.Error())
		return err
	}

	//modify DGSCol.Status.PodState
	err = c.assignPodCollectionState(dgsColToUpdate)
	if err != nil {
		c.recorder.Event(dgsColTemp, corev1.EventTypeWarning, "Error setting DedicatedGameServerCollection - Pod status", err.Error())
		return err
	}

	_, err = c.dgsColClient.DedicatedGameServerCollections(namespace).Update(dgsColToUpdate)

	if err != nil {
		log.Errorf("ERROR in updating DGS Col:%s", err.Error())
		c.recorder.Event(dgsColTemp, corev1.EventTypeWarning, "Error updating DedicatedGameServerCollection", err.Error())
		return err
	}

	c.recorder.Event(dgsColTemp, corev1.EventTypeNormal, shared.SuccessSynced, fmt.Sprintf(shared.MessageResourceSynced, "DedicatedGameServerCollection", dgsColTemp.Name))
	return nil
}

func (c *DedicatedGameServerCollectionController) modifyAvailableReplicas(dgsCol *dgsv1alpha1.DedicatedGameServerCollection) error {

	set := labels.Set{
		shared.LabelDedicatedGameServerCollectionName: dgsCol.Name,
	}
	// we seach via Labels, each DGS will have the DGSCol name as a Label
	selector := labels.SelectorFromSet(set)
	dgsInstances, err := c.dgsLister.DedicatedGameServers(dgsCol.Namespace).List(selector)

	if err != nil {
		return err
	}

	dgsCol.Status.AvailableReplicas = 0

	for _, dgs := range dgsInstances {
		if dgs.Status.GameServerState == shared.GameServerStateRunning && dgs.Status.PodState == shared.PodStateRunning {
			dgsCol.Status.AvailableReplicas++
		}
	}

	return nil
}

func (c *DedicatedGameServerCollectionController) assignGameServerCollectionState(
	dgsCol *dgsv1alpha1.DedicatedGameServerCollection) error {

	set := labels.Set{
		shared.LabelDedicatedGameServerCollectionName: dgsCol.Name,
	}
	// we seach via Labels, each DGS will have the DGSCol name as a Label
	selector := labels.SelectorFromSet(set)
	dgsInstances, err := c.dgsLister.DedicatedGameServers(dgsCol.Namespace).List(selector)

	if err != nil {
		return err
	}

	for _, dgs := range dgsInstances {
		if dgs.Status.GameServerState != shared.GameServerStateRunning {
			dgsCol.Status.GameServerCollectionState = dgs.Status.GameServerState
			return nil
		}
	}
	dgsCol.Status.GameServerCollectionState = shared.GameServerCollectionStateRunning
	return nil

}

func (c *DedicatedGameServerCollectionController) assignPodCollectionState(dgsCol *dgsv1alpha1.DedicatedGameServerCollection) error {
	// if this Pod state is running, then we should check whether
	// ALL the Pod in the Collection have the running state
	// if true, then DGSCol Pod state is running
	// else DGSCol Pod state is equal to this Pod State
	set := labels.Set{
		shared.LabelDedicatedGameServerCollectionName: dgsCol.Name,
	}
	// we seach via Labels, each DGS will have the DGSCol name as a Label
	selector := labels.SelectorFromSet(set)
	dgsInstances, err := c.dgsLister.DedicatedGameServers(dgsCol.Namespace).List(selector)

	if err != nil {
		return err
	}

	for _, dgs := range dgsInstances {
		if dgs.Status.PodState != shared.PodStateRunning {
			dgsCol.Status.PodCollectionState = dgs.Status.PodState
			return nil
		}
	}
	dgsCol.Status.PodCollectionState = shared.PodCollectionRunning
	return nil

}

func (c *DedicatedGameServerCollectionController) handleDedicatedGameServerCollection(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServerCollection object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServerCollection object tombstone, invalid type"))
			return
		}
		log.Infof("Recovered deleted DedicatedGameServerCollection object '%s' from tombstone", object.GetName())
	}

	c.enqueueDedicatedGameServerCollection(object)
}

func (c *DedicatedGameServerCollectionController) handleDedicatedGameServer(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServer object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServer object tombstone, invalid type"))
			return
		}
		log.Infof("Recovered deleted DedicatedGameServerCollection object '%s' from tombstone", object.GetName())
	}

	//if this DGS has a parent DGSCol
	if len(object.GetOwnerReferences()) > 0 {
		dgsCol, err := c.dgsColLister.DedicatedGameServerCollections(object.GetNamespace()).Get(object.GetOwnerReferences()[0].Name)
		if err != nil {
			runtime.HandleError(fmt.Errorf("error getting a DedicatedGameServer Collection from the Dedicated Game Server with Name %s", object.GetName()))
			return
		}

		c.enqueueDedicatedGameServerCollection(dgsCol)
	}
}

// enqueueDedicatedGameServerCollection takes a DedicatedGameServerCollection resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than DedicatedGameServerCollection.
func (c *DedicatedGameServerCollectionController) enqueueDedicatedGameServerCollection(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *DedicatedGameServerCollectionController) runWorker() {
	// hot loop until we're told to stop.  processNextWorkItem will
	// automatically wait until there's work available, so we don't worry
	// about secondary waits
	log.Info("Starting loop for DedicatedGameServerCollection controller")
	for c.processNextWorkItem() {
	}
}

func (c *DedicatedGameServerCollectionController) Run(controllerThreadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Info("Starting DedicatedGameServerCollection controller")

	// Wait for the caches for all controllers to be synced before starting workers
	log.Info("Waiting for informer caches to sync for DedicatedGameServerCollection controller")
	if ok := cache.WaitForCacheSync(stopCh, c.dgsColListerSynced, c.dgsListerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Info("Starting workers for DedicatedGameServerCollection controller")

	// Launch a number of workers to process resources
	for i := 0; i < controllerThreadiness; i++ {
		// runWorker will loop until "something bad" happens.  The .Until will
		// then rekick the worker after one second
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers for DedicatedGameServerCollection controller")
	<-stopCh
	log.Info("Shutting down workers for DedicatedGameServerCollection controller")

	return nil
}
