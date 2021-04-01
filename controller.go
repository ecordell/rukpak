package main

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"context"
	"fmt"
	"time"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "github.com/ecordell/rukpak/pkg/apis/rukpak/clientset/versioned"
	rukpakscheme "github.com/ecordell/rukpak/pkg/apis/rukpak/clientset/versioned/scheme"
	informers "github.com/ecordell/rukpak/pkg/apis/rukpak/informers/externalversions/rukpak/v1alpha1"
	listers "github.com/ecordell/rukpak/pkg/apis/rukpak/listers/rukpak/v1alpha1"
	rukpakv1alpha1 "github.com/ecordell/rukpak/pkg/apis/rukpak/v1alpha1"
)

const controllerAgentName = "rukpak-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Bundle is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Bundle fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Bundle"
	// MessageResourceSynced is the message used for an Event fired when a Bundle
	// is synced successfully
	MessageResourceSynced = "Bundle synced successfully"
)

// Controller is the controller implementation for Bundle resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// rukpakclientset is a clientset for our own API group
	rukpakclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	bundleLister      listers.BundleLister
	bundleSynced      cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	interpreter *Interpreter
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	rukpakclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	bundleInformer informers.BundleInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(rukpakscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		rukpakclientset:   rukpakclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		bundleLister:      bundleInformer.Lister(),
		bundleSynced:      bundleInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Bundles"),
		recorder:          recorder,
		interpreter:  	   NewInterpreter(BundleMachine.WithContext(BundleMachineContext{client: rukpakclientset})),
	}

	klog.Info("Setting up event handlers")
	//Set up an event handler for when Bundle resources change
	bundleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueBundle,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueBundle(new)
		},
	})


	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Bundle resource will enqueue that Bundle resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	//deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: controller.handleObject,
	//	UpdateFunc: func(old, new interface{}) {
	//		newDepl := new.(*appsv1.Deployment)
	//		oldDepl := old.(*appsv1.Deployment)
	//		if newDepl.ResourceVersion == oldDepl.ResourceVersion {
	//			// Periodic resync will send update events for all known Deployments.
	//			// Two different versions of the same Deployment will always have different RVs.
	//			return
	//		}
	//		controller.handleObject(new)
	//	},
	//	DeleteFunc: controller.handleObject,
	//})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Bundle controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.bundleSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Bundle resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	if err := c.interpreter.Start(threadiness, stopCh); err != nil {
		return err
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
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
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Bundle resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Bundle resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	// Get the Bundle resource with this namespace/name
	bundle, err := c.bundleLister.Bundles(namespace).Get(name)
	if err != nil {
		// TODO: bundleremoved event
		// The Bundle resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	c.interpreter.Receive(BundleSyncEvent(bundle))
	return nil
}

// enqueueBundle takes a Bundle resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *Controller) enqueueBundle(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}


type ControllerContext struct {
	kubeclientset kubernetes.Interface
	rukpakclientset clientset.Interface
	configMapInformer coreinformers.ConfigMapInformer
	bundleInformer informers.BundleInformer
	configmapLister corelisters.ConfigMapLister
	bundleLister    listers.BundleLister
	workqueue        workqueue.RateLimitingInterface
}

func NewControllerMachine(kubeclientset kubernetes.Interface,
	rukpakclientset clientset.Interface,
	configmapInformer coreinformers.ConfigMapInformer,
	bundleInformer informers.BundleInformer) *Machine {
	return Machine{
		Name: "ControllerMachine",
		InitialState: &WaitingForCachesToSync,
		States: []State{&IdleController, &WaitingForCachesToSync, &ProcessBundle, &ProcessObject},
	}.WithContext()
}

func NewControllerMachine(
	kubeclientset kubernetes.Interface,
	rukpakclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	bundleInformer informers.BundleInformer) *Machine {

	utilruntime.Must(rukpakscheme.AddToScheme(scheme.Scheme))

	//context = NewContext(map[string]interface{}{""})
	controller := &Controller{
		kubeclientset:     kubeclientset,
		rukpakclientset:   rukpakclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		bundleLister:      bundleInformer.Lister(),
		bundleSynced:      bundleInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Bundles"),
		interpreter:  	   NewInterpreter(BundleMachine.WithContext(BundleMachineContext{client: rukpakclientset})),
	}

	klog.Info("Setting up event handlers")
	//Set up an event handler for when Bundle resources change
	bundleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueBundle,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueBundle(new)
		},
	})

	return controller
}


type EventType string

type Event interface {
	Type() EventType
	Ctx() interface{}
}

type TypedEvent struct {
	EventType EventType
	Context interface{}
}

func (e TypedEvent) Type() EventType {
	return e.EventType
}

func (e TypedEvent) Ctx() interface{} {
	return e.Context
}

type Action func(interface{}, Event)

type GuardFunc func(interface{}, Event) bool

type Transition struct {
	Target  State
	Actions map[string]Action
	Gaurd   GuardFunc
}

type StateNode struct {
	Name string
	OnEnter Action
	OnExit Action
	Transitions map[EventType][]Transition
}

type Machine struct {
	Name             string
	Ctx              interface{}
	Parents          []*Machine
	ParallelChildren bool
	Children         []*Machine
	CurrentState     State
	InitialState     State
	States           []State
}

func (m *Machine) Send(event Event) {
	m.Receive(m.Ctx, event)
}

func (m Machine) WithContext(ctx interface{}) Machine {
	m.Ctx = ctx
	return *m
}

func (m *Machine) Receive(ctx interface{}, event Event) State {
	m.Ctx = ctx
	if m.CurrentState == nil {
		m.CurrentState = m.InitialState
	}
	m.CurrentState = m.CurrentState.Receive(ctx, event)

	// propagate event to child machines
	if machine, ok := m.CurrentState.(*Machine); ok {
		machine.Receive(ctx, event)
	}
	// TODO: parent propagation

	return m.CurrentState
}

func (m *Machine) ID() string {
	return m.Name
}

func (m *Machine) GetTransitions(e EventType) []Transition {
	return nil
}

func (m *Machine) Enter(ctx interface{}, event Event)  {
	fmt.Println("entering", m.ID())
	m.Ctx = ctx
	if m.CurrentState == nil {
		m.CurrentState = m.InitialState
	}
	return
}

func (m *Machine) Exit(ctx interface{}, event Event)  {
	return
}

type State interface {
	ID() string
	GetTransitions(EventType) []Transition
	Enter(interface{}, Event)
	Exit(interface{}, Event)
	Receive(interface{}, Event) State
}

var _ State = &Machine{}
var _ State = &StateNode{}

func (s *StateNode) Receive(ctx interface{}, event Event) State {
	s.Enter(ctx, event)
	defer s.Exit(ctx, event)

	fmt.Println("in", s.Name)

	// use first transition that passes the guard
	var t Transition
	for _, o := range s.GetTransitions(event.Type()) {
		if o.Gaurd == nil || o.Gaurd(ctx, event) {
			t = o
			break
		}
	}

	fmt.Println("to", t.Target.ID())

	for name, a := range t.Actions {
		fmt.Println("running action", name)
		a(ctx, event)
	}

	return t.Target
}

func (s *StateNode) ID() string {
	return s.Name
}

func (s *StateNode) GetTransitions(e EventType) []Transition {
	if s.Transitions == nil {
		return nil
	}
	ts, ok := s.Transitions[e]
	if !ok {
		return nil
	}
	return ts
}

func (s *StateNode) Enter(ctx interface{}, event Event)  {
	if s.OnEnter == nil {
		return
	}
	s.OnEnter(ctx, event)
}

func (s *StateNode) Exit(ctx interface{}, event Event)  {
	if s.OnExit == nil {
		return
	}
	s.OnExit(ctx, event)
}

type Interpreter struct {
	Machine
	workqueue workqueue.RateLimitingInterface
}

func NewInterpreter(machine Machine) *Interpreter {
	if len(machine.Parents) > 0 {
		// must start interpreter on a top-level machine
		return nil
	}
	return &Interpreter{
		Machine: machine,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "events"),
	}
}

func (m *Interpreter) Start(threadiness int, stopCh <-chan struct{}) error {
	if len(m.Machine.Parents) > 0 {
		// must start interpreter on a top-level state
		return fmt.Errorf("machine can't have parents")
	}
	for i := 0; i < threadiness; i++ {
		go wait.Until(m.runWorker, time.Second, stopCh)
	}
	return nil
}

func (m *Interpreter) runWorker() {
	for m.process() {
	}
}

func (m *Interpreter) process() bool {
	e, shutdown := m.workqueue.Get()
	if shutdown {
		return false
	}
	event, ok := e.(Event)
	if !ok {
		return true
	}

	m.Machine.Send(event)
	return true
}

func (m *Interpreter) Receive(e Event) {
	m.workqueue.Add(e)
}


var BundleSync EventType = "BundleSync"

type BundleEventContext struct {
	Bundle *rukpakv1alpha1.Bundle
}

func BundleSyncEvent(bundle *rukpakv1alpha1.Bundle) Event {
	return &TypedEvent{
		EventType: BundleSync,
		Context: BundleEventContext{Bundle: bundle},
	}
}

func GetBundleFromEvent(e Event) (*rukpakv1alpha1.Bundle, error) {
	if e.Type() != "BundleSync" {
		return nil, fmt.Errorf("can't get bundle from event of this type")
	}
	c, ok := e.Ctx().(BundleEventContext)
	if !ok {
		return nil, fmt.Errorf("wrong type found in event context")
	}
	return c.Bundle, nil
}

var ConfigMapSync EventType = "ConfigMapSync"

var CtxBundleClient = "BundleClient"

type BundleMachineContext struct {
	client clientset.Interface
}

var BundleMachine = Machine{
	Name: "BundleMachine",
	InitialState: &Idle,
	States: []State{&Idle, &BundlesPresent, &BundleRequiresCleanup},
}

var Idle = StateNode{
	Name: "Idle",
	Transitions: map[EventType][]Transition{
		BundleSync: {
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					bundle, err := GetBundleFromEvent(event)
					if err != nil {
						return false
					}
					return bundle.GetDeletionTimestamp() == nil
				},
				Target: &BundlesPresent,
			},
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					bundle, err := GetBundleFromEvent(event)
					if err != nil {
						return false
					}
					return bundle.GetDeletionTimestamp() != nil
				},
				Target: &BundleRequiresCleanup,
			},
		},
		ConfigMapSync: {},
	},
}

var BundlesPresent = Machine{
	Name: "BundlesPresent",
	InitialState: &BundleStateUnknown,
	States: []State{&BundleStateUnknown, &BundleGenerationMatches, &BundleNewGeneration},
}

var BundleStateUnknown = StateNode{
	Name: "BundleStateUnknown",
	Transitions: map[EventType][]Transition{
		BundleSync: {
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					bundle, err := GetBundleFromEvent(event)
					if err != nil {
						return false
					}
					return bundle.GetGeneration() == bundle.Status.ObservedGeneration
				},
				Target: &BundleGenerationMatches,
			},
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					bundle, err := GetBundleFromEvent(event)
					if err != nil {
						return false
					}
					return bundle.GetGeneration() != bundle.Status.ObservedGeneration
				},
				Target:  &BundleNewGeneration,
				Actions: map[string]Action{"SyncObservedGeneration": SyncObservedGenerationAction},
			},
		},
	},
}

var BundleGenerationMatches = Machine{
	Name: "BundleGenerationMatches",
	States: []State{&ContentStateUnknown, &ContentSynced, &ContentNotSynced},
	InitialState: &ContentStateUnknown,
}

var ContentStateUnknown = StateNode{
	Name: "ContentStateUnknown",
	Transitions: map[EventType][]Transition{
		BundleSync: {
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					bundle, err := GetBundleFromEvent(event)
					if err != nil {
						return false
					}
					fmt.Println(bundle.Spec)
					// if unpacked content exists
					return true
				},
				Target: &ContentSynced,
			},
			{
				Gaurd: func(ctx interface{}, event Event) bool {
					// if unpacked content does not exist
					return true
				},
				Target: &ContentNotSynced,
			},
		},
	},
}

var BundleNewGeneration = StateNode{
	Name: "BundleNewGeneration",
}

var ContentSynced = StateNode{
	Name: "ContentSynced",
}

var ContentNotSynced = StateNode{
	Name: "ContentNotSynced",
	OnEnter: func(ctx interface{}, event Event) {
		// do the unpack
	},
}

var BundleRequiresCleanup = StateNode{
	Name: "BundleRequiresCleanup",
}

func SyncObservedGenerationAction(ctx interface{}, event Event) {
	bundle, err := GetBundleFromEvent(event)
	if err != nil {
		return
	}

	machineContext, ok := ctx.(BundleMachineContext)
	if !ok {
		return
	}
	if err := SyncObservedGeneration(machineContext.client, bundle); err != nil {
		// log
	}
	return
}

func SyncObservedGeneration(client clientset.Interface, bundle *rukpakv1alpha1.Bundle) error {
	bundle.Status.ObservedGeneration = bundle.GetGeneration()
	_, err := client.RukpakV1alpha1().Bundles(bundle.GetNamespace()).UpdateStatus(context.TODO(), bundle, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	// TODO: unset unpacked objects in status when
	return nil
}


// TODO: parallel states
// TODO: context key validation
// TODO: goroutine receivers
// TODO: parent/child wiring