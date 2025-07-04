package controller

import (
	"errors"
	ficV1 "github.com/F5Networks/f5-ipam-controller/pkg/ipamapis/apis/fic/v1"
	cisapiv1 "github.com/F5Networks/k8s-bigip-ctlr/v2/config/apis/cis/v1"
	log "github.com/F5Networks/k8s-bigip-ctlr/v2/pkg/vlogger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

func (ctlr *Controller) enqueueReq(config ResourceConfigRequest) requestMeta {
	ctlr.requestCounter = ctlr.requestCounter + 1
	rm := requestMeta{
		partitionMap: make(map[string]map[string]string, len(config.ltmConfig)),
		id:           ctlr.requestCounter,
	}
	for partition, partitionConfig := range config.ltmConfig {
		rm.partitionMap[partition] = make(map[string]string)
		for _, cfg := range partitionConfig.ResourceMap {
			for key, val := range cfg.MetaData.baseResources {
				rm.partitionMap[partition][key] = val
			}
		}
	}
	return rm
}

func (ctlr *Controller) responseHandler() {
	for agentConfig := range ctlr.respChan {
		for partition, meta := range agentConfig.reqMeta.partitionMap {
			for rscKey, kind := range meta {
				ctlr.removeUnusedIPAMEntries(kind)
				ns := strings.Split(rscKey, "/")[0]
				switch kind {
				case VirtualServer:
					// update status
					crInf, ok := ctlr.getNamespacedCRInformer(ns, ctlr.multiClusterHandler.LocalClusterName)
					if !ok {
						log.Debugf("VirtualServer Informer not found for namespace: %v", ns)
						continue
					}
					obj, exist, err := crInf.vsInformer.GetIndexer().GetByKey(rscKey)
					if err != nil {
						log.Debugf("Could not fetch VirtualServer: %v: %v", rscKey, err)
						continue
					}
					if !exist {
						log.Debugf("VirtualServer Not Found: %v", rscKey)
						continue
					}
					virtual := obj.(*cisapiv1.VirtualServer)
					ip, _ := ctlr.ResourceStatusVSAddressMap[resourceRef{
						name:      virtual.Name,
						namespace: virtual.Namespace,
						kind:      VirtualServer,
					}]
					if virtual.Namespace+"/"+virtual.Name == rscKey {
						if tenantResponse, found := agentConfig.failedTenants[partition]; found {
							// update the status for virtual server as tenant posting is failed
							ctlr.updateVSStatus(virtual, ip, StatusError, errors.New(tenantResponse.message))
						} else {
							// update the status for virtual server as tenant posting is success
							ctlr.updateVSStatus(virtual, ip, StatusOk, nil)
							// Update Corresponding Service Status of Type LB
							if !ctlr.isAddingPoolRestricted(ctlr.multiClusterHandler.LocalClusterName) {
								// set status of all the LB services associated with this VS
								go ctlr.updateLBServiceStatusForVSorTS(virtual, ip, true)
							}
						}
					}

				case TransportServer:
					// update status
					crInf, ok := ctlr.getNamespacedCRInformer(ns, ctlr.multiClusterHandler.LocalClusterName)
					if !ok {
						log.Debugf("TransportServer Informer not found for namespace: %v", ns)
						continue
					}
					obj, exist, err := crInf.tsInformer.GetIndexer().GetByKey(rscKey)
					if err != nil {
						log.Debugf("Could not fetch TransportServer: %v: %v", rscKey, err)
						continue
					}
					if !exist {
						log.Debugf("TransportServer Not Found: %v", rscKey)
						continue
					}
					virtual := obj.(*cisapiv1.TransportServer)

					ip, _ := ctlr.ResourceStatusVSAddressMap[resourceRef{
						name:      virtual.Name,
						namespace: virtual.Namespace,
						kind:      TransportServer,
					}]
					if virtual.Namespace+"/"+virtual.Name == rscKey {
						if tenantResponse, found := agentConfig.failedTenants[partition]; found {
							// update the status for transport server as tenant posting is failed
							ctlr.updateTSStatus(virtual, ip, StatusError, errors.New(tenantResponse.message))
						} else {
							// update the status for transport server as tenant posting is success
							ctlr.updateTSStatus(virtual, ip, StatusOk, nil)
							// set status of all the LB services associated with this TS
							go ctlr.updateLBServiceStatusForVSorTS(virtual, ip, true)
						}
					}

				case IngressLink:
					// update status
					crInf, ok := ctlr.getNamespacedCRInformer(ns, ctlr.multiClusterHandler.LocalClusterName)
					if !ok {
						log.Debugf("IngressLink Informer not found for namespace: %v", ns)
						continue
					}
					obj, exist, err := crInf.ilInformer.GetIndexer().GetByKey(rscKey)
					if err != nil {
						log.Debugf("Could not fetch IngressLink: %v: %v", rscKey, err)
						continue
					}
					if !exist {
						log.Debugf("IngressLink Not Found: %v", rscKey)
						continue
					}
					il := obj.(*cisapiv1.IngressLink)

					ip, _ := ctlr.ResourceStatusVSAddressMap[resourceRef{
						name:      il.Name,
						namespace: il.Namespace,
						kind:      IngressLink,
					}]
					if il.Namespace+"/"+il.Name == rscKey {
						if tenantResponse, found := agentConfig.failedTenants[partition]; found {
							// update the status for ingresslink as tenant posting is failed
							ctlr.updateILStatus(il, ip, StatusError, errors.New(tenantResponse.message))
						} else {
							// update the status for ingresslink as tenant posting is success
							ctlr.updateILStatus(il, ip, StatusOk, nil)
						}
					}

				case Route:
					if _, found := agentConfig.failedTenants[partition]; found {
						// TODO : distinguish between a 503 and an actual failure
						go ctlr.updateRouteAdmitStatus(rscKey, "Failure while updating config", "Please check logs for more information", v1.ConditionFalse)
					} else {
						go ctlr.updateRouteAdmitStatus(rscKey, "", "", v1.ConditionTrue)
					}
				}
			}
		}
		switch agentConfig.agentKind {
		case PrimaryBigIP:
			if !ctlr.RequestHandler.PrimaryBigIPWorker.disableARP {
				go ctlr.RequestHandler.PrimaryBigIPWorker.updateARPsForPoolMembers(agentConfig.rscConfigRequest)
			}
			// If GTM is running on separate server with CCCLGTMAgent set as true, then Primary worker will post the GTM config on the GTM server.
			// post to the BIGIP if CCCLGTMAgent is set to true. It will only update GTM config on the GTM server wheather running on same server or different server.
			if ctlr.RequestHandler.PrimaryBigIPWorker.ccclGTMAgent {
				log.Debugf("%v Posting GTM config to cccl agent: %+v\n", ctlr.RequestHandler.PrimaryBigIPWorker.APIHandler.LTM.postManagerPrefix, agentConfig.rscConfigRequest)
				ctlr.RequestHandler.PrimaryBigIPWorker.PostGTMConfigWithCccl(agentConfig.rscConfigRequest)
			}
		case SecondaryBigIP:
			if !ctlr.RequestHandler.SecondaryBigIPWorker.disableARP {
				go ctlr.RequestHandler.SecondaryBigIPWorker.updateARPsForPoolMembers(agentConfig.rscConfigRequest)
			}
			// If GTM is running on separate server with CCCLGTMAgent set as true, then Primary worker will post the GTM config on the GTM server.
			// We don't want to send the duplicate requests on separate GTM server using both agents.
			// post to the BIGIP if CCCLGTMAgent is set to true and GTM is not running on same server. It will only update GTM config when GTM running on same server.
			if ctlr.RequestHandler.SecondaryBigIPWorker.ccclGTMAgent && !isGTMOnSeparateServer(ctlr.RequestHandler.agentParams) {
				log.Debugf("%v Posting GTM config to cccl agent: %+v\n", ctlr.RequestHandler.SecondaryBigIPWorker.APIHandler.LTM.postManagerPrefix, agentConfig.rscConfigRequest)
				ctlr.RequestHandler.SecondaryBigIPWorker.PostGTMConfigWithCccl(agentConfig.rscConfigRequest)
			}
		}
		// anonymous function to handle the failure timeouts
		if len(agentConfig.failedTenants) > 0 {
			go func(agentConfig *agentPostConfig) {
				// Delay the retry of failed tenants
				<-time.After(agentConfig.timeout)
				if ctlr.requestCounter == agentConfig.reqMeta.id {
					switch agentConfig.agentKind {
					case GTMBigIP:
						ctlr.RequestHandler.GTMBigIPWorker.getPostManager().postChan <- agentConfig
					case PrimaryBigIP:
						ctlr.RequestHandler.PrimaryBigIPWorker.getPostManager().postChan <- agentConfig
					case SecondaryBigIP:
						ctlr.RequestHandler.SecondaryBigIPWorker.getPostManager().postChan <- agentConfig
					}
				}
			}(agentConfig)
		}

	}
}

func (ctlr *Controller) removeUnusedIPAMEntries(kind string) {
	// Remove Unused IPAM entries in IPAM CR after CIS restarts, applicable to only first PostCall
	if !ctlr.firstPostResponse && ctlr.ipamCli != nil && (kind == VirtualServer || kind == TransportServer) {
		ctlr.firstPostResponse = true
		toRemoveIPAMEntries := &ficV1.IPAM{
			ObjectMeta: metav1.ObjectMeta{
				Labels: make(map[string]string),
			},
		}
		ipamCR := ctlr.getIPAMCR()
		for _, hostSpec := range ipamCR.Spec.HostSpecs {
			found := false
			ctlr.cacheIPAMHostSpecs.Lock()
			for cacheIndex, cachehostSpec := range ctlr.cacheIPAMHostSpecs.IPAM.Spec.HostSpecs {
				if (hostSpec.IPAMLabel == cachehostSpec.IPAMLabel && hostSpec.Host == cachehostSpec.Host) ||
					(hostSpec.IPAMLabel == cachehostSpec.IPAMLabel && hostSpec.Key == cachehostSpec.Key) ||
					(hostSpec.IPAMLabel == cachehostSpec.IPAMLabel && hostSpec.Key == cachehostSpec.Key && hostSpec.Host == cachehostSpec.Host) {
					if len(ctlr.cacheIPAMHostSpecs.IPAM.Spec.HostSpecs) > cacheIndex {
						ctlr.cacheIPAMHostSpecs.IPAM.Spec.HostSpecs = append(ctlr.cacheIPAMHostSpecs.IPAM.Spec.HostSpecs[:cacheIndex], ctlr.cacheIPAMHostSpecs.IPAM.Spec.HostSpecs[cacheIndex+1:]...)
					}
					found = true
					break
				}
			}
			ctlr.cacheIPAMHostSpecs.Unlock()
			if !found {
				// To remove
				toRemoveIPAMEntries.Spec.HostSpecs = append(toRemoveIPAMEntries.Spec.HostSpecs, hostSpec)
			}
		}
		for _, removeIPAMentry := range toRemoveIPAMEntries.Spec.HostSpecs {
			ipamCR = ctlr.getIPAMCR()
			for index, hostSpec := range ipamCR.Spec.HostSpecs {
				if (hostSpec.IPAMLabel == removeIPAMentry.IPAMLabel && hostSpec.Host == removeIPAMentry.Host) ||
					(hostSpec.IPAMLabel == removeIPAMentry.IPAMLabel && hostSpec.Key == removeIPAMentry.Key) ||
					(hostSpec.IPAMLabel == removeIPAMentry.IPAMLabel && hostSpec.Key == removeIPAMentry.Key && hostSpec.Host == removeIPAMentry.Host) {
					_, err := ctlr.RemoveIPAMCRHostSpec(ipamCR, removeIPAMentry.Key, index)
					if err != nil {
						log.Errorf("[IPAM] ipam hostspec update error: %v", err)
					}
					break
				}
			}
		}
		// Delete cacheIPAMHostSpecs
		ctlr.cacheIPAMHostSpecs = CacheIPAM{}
	}
}
