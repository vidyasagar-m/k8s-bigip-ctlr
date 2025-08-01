package controller

import (
	"context"
	ficV1 "github.com/F5Networks/f5-ipam-controller/pkg/ipamapis/apis/fic/v1"
	cisapiv1 "github.com/F5Networks/k8s-bigip-ctlr/v2/config/apis/cis/v1"
	crdfake "github.com/F5Networks/k8s-bigip-ctlr/v2/config/client/clientset/versioned/fake"
	"github.com/F5Networks/k8s-bigip-ctlr/v2/pkg/teem"
	"github.com/F5Networks/k8s-bigip-ctlr/v2/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routeapi "github.com/openshift/api/route/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("Informers Tests", func() {
	var mockCtlr *mockController
	var mockPM *mockPostManager
	namespace := "default"

	BeforeEach(func() {
		mockCtlr = newMockController()
		mockWriter := &test.MockWriter{}
		mockCtlr.RequestHandler = newMockRequestHandler(mockWriter)
		mockPM = newMockPostManger()
		mockPM.TokenManagerInterface = test.NewMockTokenManager("test-token")
		mockPM.BIGIPURL = "bigip.com"
		mockCtlr.RequestHandler.PrimaryBigIPWorker.LTM.PostManager = mockPM.PostManager
		mockCtlr.multiClusterHandler = NewClusterHandler("")
		mockCtlr.webhookServer = &mockWebHookServer{}
		go mockCtlr.multiClusterHandler.ResourceEventWatcher()
		// Handles the resource status updates
		go mockCtlr.multiClusterHandler.ResourceStatusUpdater()
	})

	Describe("Custom Resource Informers", func() {
		BeforeEach(func() {
			mockCtlr.mode = CustomResourceMode
			mockCtlr.multiClusterHandler.ClusterConfigs[""] = newClusterConfig()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces = make(map[string]struct{})
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces["default"] = struct{}{}
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeClient = k8sfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeCRClient = crdfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].InformerStore = initInformerStore()
			mockCtlr.multiClusterHandler.customResourceSelector, _ = createLabelSelector(DefaultCustomResourceLabel)
		})
		It("Resource Informers", func() {
			err := mockCtlr.addNamespacedInformers(namespace, false, "")
			Expect(err).To(BeNil(), "Informers Creation Failed")

			crInf, found := mockCtlr.getNamespacedCRInformer(namespace, "")
			Expect(crInf).ToNot(BeNil(), "Finding Informer Failed")
			Expect(found).To(BeTrue(), "Finding Informer Failed")
		})

		It("Namespace Informer", func() {
			namespaceSelector, err := createLabelSelector("app=test")
			Expect(namespaceSelector).ToNot(BeNil(), "Failed to Create Label Selector")
			Expect(err).To(BeNil(), "Failed to Create Label Selector")
			err = mockCtlr.createNamespaceLabeledInformerForCluster("app=test", "")
			Expect(err).To(BeNil(), "Failed to Create Namespace Informer")
		})
	})

	Describe("Custom Resource Queueing", func() {
		BeforeEach(func() {
			mockCtlr.mode = CustomResourceMode
			mockCtlr.multiClusterHandler.ClusterConfigs[""] = newClusterConfig()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces["default"] = struct{}{}
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeClient = k8sfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeCRClient = crdfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].InformerStore = initInformerStore()
			mockCtlr.multiClusterHandler.customResourceSelector, _ = createLabelSelector(DefaultCustomResourceLabel)
			mockCtlr.resourceQueue = workqueue.NewNamedRateLimitingQueue(
				workqueue.DefaultControllerRateLimiter(), "custom-resource-controller")
			mockCtlr.resources = NewResourceStore()
			mockCtlr.resources.ltmConfig = make(map[string]*PartitionConfig, 0)
			mockCtlr.Partition = "test"
			mockCtlr.TeemData = &teem.TeemsData{
				ResourceType: teem.ResourceTypes{
					VirtualServer:   make(map[string]int),
					TransportServer: make(map[string]int),
				},
			}
			mockCtlr.ResourceStatusVSAddressMap = make(map[resourceRef]string)
			mockCtlr.multiClusterResources = newMultiClusterResourceStore()

		})
		AfterEach(func() {
			mockCtlr.resourceQueue.ShutDown()
		})
		It("VirtualServer", func() {
			vs := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "1.2.3.4",
				})
			mockCtlr.enqueueVirtualServer(vs)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue New VS  Failed")

			newVS := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "1.2.3.5",
					Partition:            "dev",
				})
			zero := 0
			mockCtlr.resources.ltmConfig[mockCtlr.Partition] = &PartitionConfig{ResourceMap: make(ResourceMap), Priority: &zero}
			mockCtlr.enqueueUpdatedVirtualServer(vs, newVS)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated VS  Failed")
			Expect(*mockCtlr.resources.ltmConfig[mockCtlr.Partition].Priority).To(BeEquivalentTo(1), "Priority Not Updated")
			delete(mockCtlr.resources.ltmConfig, mockCtlr.Partition)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated VS  Failed")

			mockCtlr.enqueueDeletedVirtualServer(newVS)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted VS  Failed")

			// Check if correct event in set while enqueuing vs
			// Create updated VS CR
			updatedVS1 := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "1.2.3.5",
					Partition:            "dev",
					SNAT:                 "none",
				})
			mockCtlr.enqueueUpdatedVirtualServer(newVS, updatedVS1)
			// With a change of snat in VS CR, an update event should be enqueued
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated VS  Failed")
			rKey := key.(*rqKey)
			Expect(rKey).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(rKey.event).To(Equal(Update), "Incorrect event set")

			// When VirtualServerAddress is updated then it should enqueue both delete & create events
			updatedVS2 := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "5.6.7.8",
					SNAT:                 "none",
				})
			mockCtlr.enqueueUpdatedVirtualServer(updatedVS1, updatedVS2)
			key, quit = mockCtlr.resourceQueue.Get()
			// Delete event
			Expect(key).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated VS  Failed")
			rKey = key.(*rqKey)
			Expect(rKey).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(rKey.event).To(Equal(Delete), "Incorrect event set")
			// Create event
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated VS  Failed")
			rKey = key.(*rqKey)
			Expect(rKey).ToNot(BeNil(), "Enqueue Updated VS Failed")
			Expect(rKey.event).To(Equal(Create), "Incorrect event set")

			mockCtlr.enqueueVirtualServer(vs)
			Expect(mockCtlr.processResources()).To(Equal(true))

			// Verify VS status update event is not queued for processing
			updatedStatusVS := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "5.6.7.8",
					SNAT:                 "none",
				})
			updatedStatusVS.Status.Status = StatusOk
			mockCtlr.enqueueUpdatedVirtualServer(updatedVS2, updatedStatusVS)
			Expect(mockCtlr.resourceQueue.Len()).To(Equal(0), "VS status update should be skipped")

			// Verify VS Label update event is queued for processing
			updatedLabelVS := test.NewVirtualServer(
				"SampleVS",
				namespace,
				cisapiv1.VirtualServerSpec{
					Host:                 "test.com",
					VirtualServerAddress: "5.6.7.8",
					SNAT:                 "none",
				})
			labels := make(map[string]string)
			labels["f5cr"] = "false"
			updatedLabelVS.Labels = labels
			mockCtlr.enqueueUpdatedVirtualServer(updatedStatusVS, updatedLabelVS)
			Expect(mockCtlr.resourceQueue.Len()).To(Equal(1), "VS label update should not be skipped")
		})

		It("TLS Profile", func() {
			tlsp := test.NewTLSProfile(
				"SampleTLS",
				namespace,
				cisapiv1.TLSProfileSpec{
					Hosts: []string{"test.com", "prod.com"},
					TLS: cisapiv1.TLS{
						Termination: "edge",
						ClientSSL:   "2359qhfniqlur89phuf;rhfi",
					},
				})
			mockCtlr.enqueueTLSProfile(tlsp, Create)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue TLS Failed")
			Expect(quit).To(BeFalse(), "Enqueue TLS  Failed")

			mockCtlr.enqueueTLSProfile(tlsp, Create)
			Expect(mockCtlr.processResources()).To(Equal(true))
		})

		It("TransportServer", func() {
			ts := test.NewTransportServer(
				"SampleTS",
				namespace,
				cisapiv1.TransportServerSpec{
					SNAT:                 "auto",
					VirtualServerAddress: "1.2.3.4",
					Pool: cisapiv1.TSPool{
						Service:     "svc-1",
						ServicePort: intstr.IntOrString{IntVal: DEFAULT_HTTP_PORT},
					},
				})
			mockCtlr.enqueueTransportServer(ts)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New TS Failed")
			Expect(quit).To(BeFalse(), "Enqueue New TS  Failed")

			newTS := test.NewTransportServer(
				"SampleTS",
				namespace,
				cisapiv1.TransportServerSpec{
					SNAT:                 "auto",
					VirtualServerAddress: "1.2.3.5",
					Pool: cisapiv1.TSPool{
						Service:     "svc-1",
						ServicePort: intstr.IntOrString{IntVal: DEFAULT_HTTP_PORT},
					},
				})
			mockCtlr.enqueueUpdatedTransportServer(ts, newTS)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated TS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated TS  Failed")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated TS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated TS  Failed")

			mockCtlr.enqueueDeletedTransportServer(newTS)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted TS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted TS  Failed")

			mockCtlr.enqueueTransportServer(ts)
			Expect(mockCtlr.processResources()).To(Equal(true))
			tsWithPartition := newTS.DeepCopy()
			tsWithPartition.Spec.Partition = "dev"
			zero := 0
			mockCtlr.resources.ltmConfig[mockCtlr.Partition] = &PartitionConfig{ResourceMap: make(ResourceMap), Priority: &zero}
			mockCtlr.enqueueUpdatedTransportServer(newTS, tsWithPartition)
			Expect(*mockCtlr.resources.ltmConfig[mockCtlr.Partition].Priority).To(BeEquivalentTo(1), "Priority Not Updated")

			// Verify TS status update event is not queued for processing
			queueLen := mockCtlr.resourceQueue.Len()
			updatedStatusTS := tsWithPartition.DeepCopy()
			updatedStatusTS.Status.Status = StatusOk
			mockCtlr.enqueueUpdatedTransportServer(tsWithPartition, updatedStatusTS)
			Expect(mockCtlr.resourceQueue.Len()).To(Equal(queueLen), "TS status update should be skipped")

			// Verify TS Label update event is queued for processing
			updatedLabelTS := updatedStatusTS.DeepCopy()
			labels := make(map[string]string)
			labels["f5cr"] = "false"
			updatedLabelTS.Labels = labels
			mockCtlr.enqueueUpdatedTransportServer(updatedStatusTS, updatedLabelTS)
			Expect(mockCtlr.resourceQueue.Len()).To(Equal(queueLen+1), "TS label update should not be skipped")

		})

		It("IngressLink", func() {
			label1 := make(map[string]string)
			label1["app"] = "ingresslink"

			selctor := &metav1.LabelSelector{
				MatchLabels: label1,
			}

			iRules := []string{"dummyiRule"}
			il := test.NewIngressLink(
				"SampleIL",
				namespace,
				"1",
				cisapiv1.IngressLinkSpec{
					VirtualServerAddress: "1.2.3.4",
					Selector:             selctor,
					IRules:               iRules,
				},
			)
			mockCtlr.enqueueIngressLink(il)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New IL Failed")
			Expect(quit).To(BeFalse(), "Enqueue New IL  Failed")

			newIL := test.NewIngressLink(
				"SampleIL",
				namespace,
				"1",
				cisapiv1.IngressLinkSpec{
					VirtualServerAddress: "1.2.3.5",
					Selector:             selctor,
					IRules:               iRules,
				},
			)
			mockCtlr.enqueueUpdatedIngressLink(il, newIL)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated IL Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated IL  Failed")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated IL Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated IL  Failed")

			mockCtlr.enqueueDeletedIngressLink(newIL)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted IL Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted IL  Failed")

			mockCtlr.enqueueIngressLink(il)
			Expect(mockCtlr.processResources()).To(Equal(true))

			ilWithPartition := newIL.DeepCopy()
			ilWithPartition.Spec.Partition = "dev"
			zero := 0
			mockCtlr.resources.ltmConfig[mockCtlr.Partition] = &PartitionConfig{ResourceMap: make(ResourceMap), Priority: &zero}
			mockCtlr.enqueueUpdatedIngressLink(newIL, ilWithPartition)
			Expect(*mockCtlr.resources.ltmConfig[mockCtlr.Partition].Priority).To(BeEquivalentTo(1), "Priority Not Updated")

		})

		It("ExternalDNS", func() {
			edns := test.NewExternalDNS(
				"SampleEDNS",
				namespace,
				cisapiv1.ExternalDNSSpec{
					DomainName:        "test.com",
					LoadBalanceMethod: "round-robin",
				})
			mockCtlr.enqueueExternalDNS(edns, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New EDNS Failed")
			Expect(quit).To(BeFalse(), "Enqueue New EDNS  Failed")

			newEDNS := test.NewExternalDNS(
				"SampleEDNS",
				namespace,
				cisapiv1.ExternalDNSSpec{
					DomainName:        "prod.com",
					LoadBalanceMethod: "round-robin",
				})
			mockCtlr.enqueueUpdatedExternalDNS(edns, newEDNS, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated EDNS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated EDNS  Failed")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated EDNS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated EDNS  Failed")

			mockCtlr.enqueueDeletedExternalDNS(newEDNS, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted EDNS Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted EDNS  Failed")

			mockCtlr.TeemData = &teem.TeemsData{
				ResourceType: teem.ResourceTypes{
					RouteGroups:  make(map[string]int),
					NativeRoutes: make(map[string]int),
					ExternalDNS:  make(map[string]int),
				},
			}
			mockWriter := &test.MockWriter{FailStyle: test.Success}
			mockCtlr.RequestHandler = newMockRequestHandler(mockWriter)
			mockCtlr.Partition = "default"
			mockCtlr.enqueueExternalDNS(edns, mockCtlr.multiClusterHandler.LocalClusterName)
			Expect(mockCtlr.processResources()).To(Equal(true))
		})

		It("Policy", func() {
			plc := test.NewPolicy(
				"SamplePolicy",
				namespace,
				cisapiv1.PolicySpec{})
			mockCtlr.enqueuePolicy(plc, Create, "")
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Policy Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Policy  Failed")

			newPlc := test.NewPolicy(
				"SamplePolicy2",
				namespace,
				cisapiv1.PolicySpec{})
			mockCtlr.enqueueDeletedPolicy(newPlc, "")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated Policy Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated Policy  Failed")

			mockCtlr.enqueuePolicy(plc, Create, "")
			Expect(mockCtlr.processResources()).To(Equal(true))
		})
		It("Primary Cluster Down Event", func() {
			mockCtlr.enqueuePrimaryClusterProbeEvent()
			key, _ := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Primary cluster event key not enqueued")
		})

		It("Service", func() {
			// setting teem data
			mockCtlr.TeemData = &teem.TeemsData{
				ResourceType: teem.ResourceTypes{
					IPAMSvcLB:   make(map[string]int),
					IngressLink: make(map[string]int),
				},
			}
			svc := test.NewService(
				"SampleSVC",
				"1",
				namespace,
				v1.ServiceTypeLoadBalancer,
				nil,
			)
			mockCtlr.enqueueService(svc, "")
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Service Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Service  Failed")

			newSVC := test.NewService(
				"SampleSVC",
				"2",
				namespace,
				v1.ServiceTypeNodePort,
				nil,
			)
			mockCtlr.enqueueUpdatedService(svc, newSVC, "")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated Service Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated Service  Failed")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated Service Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated Service  Failed")

			mockCtlr.enqueueDeletedService(newSVC, "")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted Service Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted Service  Failed")

			mockCtlr.enqueueService(svc, "")
			Expect(mockCtlr.processResources()).To(Equal(true))

			svc.Name = "kube-dns"
			mockCtlr.enqueueDeletedService(svc, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Service")

			mockCtlr.enqueueUpdatedService(svc, svc, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Service")

			mockCtlr.enqueueService(svc, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Service")
		})

		It("Endpoints", func() {
			eps := test.NewEndpoints(
				"SampleSVC",
				"1",
				"worker1",
				namespace,
				[]string{"10.20.30.40"},
				nil,
				[]v1.EndpointPort{
					{
						Name: "port1",
						Port: 80,
					},
				},
			)
			mockCtlr.enqueueEndpoints(eps, Create, "")
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Endpoints Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Endpoints  Failed")

			mockCtlr.enqueueEndpoints(eps, Create, "")
			Expect(mockCtlr.processResources()).To(Equal(true))

			eps.Name = "kube-dns"
			mockCtlr.enqueueEndpoints(eps, Create, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Endpoint")
		})

		It("Pod", func() {
			label1 := make(map[string]string)
			label1["app"] = "sampleSVC"
			pod := test.NewPod(
				"SampleSVC",
				namespace,
				80,
				label1,
			)
			mockCtlr.enqueuePod(pod, "")
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Pod Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Pod Failed")

			mockCtlr.enqueueDeletedPod(pod, "")
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted Pod Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted Pod Failed")

			mockCtlr.enqueuePod(pod, "")
			Expect(mockCtlr.processResources()).To(Equal(true))

			pod.Labels["app"] = "kube-dns"
			mockCtlr.enqueuePod(pod, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Pod")
			// Verify CIS handles DeletedFinalStateUnknown pod object
			mockCtlr.enqueueDeletedPod(cache.DeletedFinalStateUnknown{Key: pod.Namespace + "/" + pod.Name, Obj: pod}, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Pod")

			// Verify CIS handles DeletedFinalStateUnknown pod object in case it doesn't have any pod Obj referenced
			mockCtlr.enqueueDeletedPod(cache.DeletedFinalStateUnknown{Key: pod.Namespace + "/" + pod.Name, Obj: nil}, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Pod")

			// Verify CIS handles scenarios when unexpected objects are received in pod deletion event
			mockCtlr.enqueueDeletedPod(nil, "")
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Invalid Pod")

		})

		It("Secret", func() {
			secret := test.NewSecret(
				"SampleSecret",
				namespace,
				"testcert",
				"testkey",
			)
			mockCtlr.enqueueSecret(secret, Create, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Secret Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Secret Failed")

			mockCtlr.enqueueSecret(secret, Create, mockCtlr.multiClusterHandler.LocalClusterName)
			Expect(mockCtlr.processResources()).To(Equal(true))
		})

		It("Namespace", func() {
			labels := make(map[string]string)
			labels["app"] = "test"
			ns := test.NewNamespace(
				"SampleNS",
				"1",
				labels,
			)
			mockCtlr.enqueueNamespace(ns, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New Namespace Failed")
			Expect(quit).To(BeFalse(), "Enqueue New Namespace  Failed")

			mockCtlr.enqueueDeletedNamespace(ns, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted Namespace Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted Namespace  Failed")

			//mockCtlr.enqueueNamespace(ns)
			//Expect(mockCtlr.processResources()).To(Equal(true))
		})

		It("IPAM", func() {
			mockCtlr.ipamCR = "default/SampleIPAM"

			hostSpec := &ficV1.HostSpec{
				Host:      "test.com",
				IPAMLabel: "test",
			}
			ipam := test.NewIPAM(
				"SampleIPAM",
				namespace,
				ficV1.IPAMSpec{
					HostSpecs: []*ficV1.HostSpec{hostSpec},
				},
				ficV1.IPAMStatus{},
			)
			mockCtlr.enqueueIPAM(ipam)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue New IPAM Failed")
			Expect(quit).To(BeFalse(), "Enqueue New IPAM  Failed")

			ipSpec := &ficV1.IPSpec{
				Host:      "test.com",
				IPAMLabel: "test",
				IP:        "1.2.3.4",
			}
			newIPAM := test.NewIPAM(
				"SampleIPAM",
				namespace,
				ficV1.IPAMSpec{
					HostSpecs: []*ficV1.HostSpec{hostSpec},
				},
				ficV1.IPAMStatus{
					IPStatus: []*ficV1.IPSpec{ipSpec},
				},
			)
			mockCtlr.enqueueUpdatedIPAM(ipam, newIPAM)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Updated IPAM Failed")
			Expect(quit).To(BeFalse(), "Enqueue Updated IPAM  Failed")

			mockCtlr.enqueueDeletedIPAM(newIPAM)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Deleted IPAM Failed")
			Expect(quit).To(BeFalse(), "Enqueue Deleted IPAM  Failed")

			mockCtlr.enqueueIPAM(ipam)
			Expect(mockCtlr.processResources()).To(Equal(true))

			newIPAM.Namespace = "test"
			mockCtlr.enqueueDeletedIPAM(newIPAM)
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Enqueue Deleted IPAM Failed")
			mockCtlr.enqueueIPAM(newIPAM)
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Enqueue Deleted IPAM Failed")
			mockCtlr.enqueueUpdatedIPAM(newIPAM, newIPAM)
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Enqueue Deleted IPAM Failed")
			Expect(mockCtlr.getEventHandlerForIPAM()).ToNot(BeNil())
		})
	})

	Describe("Common Resource Informers", func() {
		BeforeEach(func() {
			mockCtlr.mode = OpenShiftMode
			mockCtlr.multiClusterHandler.ClusterConfigs[""] = newClusterConfig()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces = make(map[string]struct{})
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces["default"] = struct{}{}
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeClient = k8sfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeCRClient = crdfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].InformerStore = initInformerStore()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].nativeResourceSelector, _ = createLabelSelector(DefaultNativeResourceLabel)
			mockCtlr.resources = NewResourceStore()
		})
		It("Resource Informers", func() {
			err := mockCtlr.addNamespacedInformers(namespace, false, "")
			Expect(err).To(BeNil(), "Informers Creation Failed")
			comInf, found := mockCtlr.getNamespacedCommonInformer(mockCtlr.multiClusterHandler.LocalClusterName, namespace)
			Expect(comInf).ToNot(BeNil(), "Finding Informer Failed")
			Expect(found).To(BeTrue(), "Finding Informer Failed")
			mockCtlr.multiClusterHandler.ClusterConfigs[""].comInformers[""] = mockCtlr.newNamespacedCommonResourceInformer("", "")
			comInf, found = mockCtlr.getNamespacedCommonInformer(mockCtlr.multiClusterHandler.LocalClusterName, namespace)
			Expect(comInf).ToNot(BeNil(), "Finding Informer Failed")
			Expect(found).To(BeTrue(), "Finding Informer Failed")
			nsObj := v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeClient.CoreV1().Namespaces().Create(context.TODO(), &nsObj, metav1.CreateOptions{})
			ns := mockCtlr.getWatchingNamespaces(mockCtlr.multiClusterHandler.LocalClusterName)
			Expect(ns).ToNot(BeNil())
			mockCtlr.multiClusterHandler.ClusterConfigs[""].nrInformers[""] = mockCtlr.newNamespacedNativeResourceInformer("")
			nrInr, found := mockCtlr.getNamespacedNativeInformer(namespace)
			Expect(nrInr).ToNot(BeNil(), "Finding Informer Failed")
			Expect(found).To(BeTrue(), "Finding Informer Failed")
		})
	})

	Describe("Native Resource Queueing", func() {
		BeforeEach(func() {
			mockCtlr.mode = OpenShiftMode
			mockCtlr.multiClusterHandler.ClusterConfigs[""] = newClusterConfig()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].namespaces["default"] = struct{}{}
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeClient = k8sfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].kubeCRClient = crdfake.NewSimpleClientset()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].InformerStore = initInformerStore()
			mockCtlr.multiClusterHandler.ClusterConfigs[""].nativeResourceSelector, _ = createLabelSelector(DefaultNativeResourceLabel)
			mockCtlr.resourceQueue = workqueue.NewNamedRateLimitingQueue(
				workqueue.DefaultControllerRateLimiter(), "native-resource-controller")
			mockCtlr.resources = NewResourceStore()
		})
		AfterEach(func() {
			mockCtlr.resourceQueue.ShutDown()
		})

		It("Route", func() {
			rt := test.NewRoute(
				"sampleroute",
				"v1",
				namespace,
				routeapi.RouteSpec{
					Host: "foo.com",
					Path: "bar",
				},
				nil)
			mockCtlr.enqueueRoute(rt, Create)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Route Failed")
			Expect(quit).To(BeFalse(), "Enqueue Route  Failed")

			mockCtlr.enqueueRoute(rt, Create)
			Expect(mockCtlr.processResources()).To(Equal(true))

			rtNew := rt.DeepCopy()
			mockCtlr.enqueueUpdatedRoute(rt, rtNew)
			Expect(mockCtlr.resourceQueue.Len()).To(BeEquivalentTo(0), "Duplicate Route Enqueued")

			rtNew.Spec.Host = "foo1.com"
			mockCtlr.enqueueUpdatedRoute(rt, rtNew)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Update Route Failed")
			Expect(quit).To(BeFalse(), "Enqueue Update Route  Failed")
			//mockCtlr.enqueueDeletedRoute(rt)
			//key, quit = mockCtlr.resourceQueue.Get()
			//Expect(key).ToNot(BeNil(), "Enqueue Route Failed")
			//Expect(quit).To(BeFalse(), "Enqueue Route  Failed")
		})

		// we are not validating this while enqueuing
		//It("Invalid ConfigMap", func() {
		//	cm := test.NewConfigMap(
		//		"samplecfgmap",
		//		"v1",
		//		namespace,
		//		map[string]string{
		//			"extendedSpec": "extendedRouteSpec",
		//		},
		//	)
		//	mockCtlr.enqueueConfigmap(cm)
		//	len := mockCtlr.resourceQueue.Len()
		//	Expect(len).To(BeZero(), "Invalid ConfigMap enqueued")
		//})

		It("Global ConfigMap", func() {
			cmName := "samplecfgmap"
			mockCtlr.globalExtendedCMKey = namespace + "/" + cmName
			cm := test.NewConfigMap(
				cmName,
				"v1",
				namespace,
				map[string]string{
					"extendedSpec": "extendedRouteSpec",
				},
			)
			mockCtlr.enqueueConfigmap(cm, Create, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Global ConfigMap Failed")
			Expect(quit).To(BeFalse(), "Enqueue Global ConfigMap  Failed")

			mockCtlr.enqueueConfigmap(cm, Delete, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Delete Global ConfigMap Failed")
			Expect(quit).To(BeFalse(), "Enqueue Delete Global ConfigMap  Failed")

			mockCtlr.enqueueDeletedConfigmap(cm, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit = mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Delete Global ConfigMap Failed")
		})

		It("Local ConfigMap", func() {
			cm := test.NewConfigMap(
				"samplecfgmap",
				"v1",
				namespace,
				map[string]string{
					"extendedSpec": "extendedRouteSpec",
				},
			)
			cm.SetLabels(map[string]string{
				"f5nr": "true",
			})
			mockCtlr.enqueueConfigmap(cm, Update, mockCtlr.multiClusterHandler.LocalClusterName)
			key, quit := mockCtlr.resourceQueue.Get()
			Expect(key).ToNot(BeNil(), "Enqueue Local ConfigMap Failed")
			Expect(quit).To(BeFalse(), "Enqueue Local ConfigMap  Failed")
		})

	})

})
