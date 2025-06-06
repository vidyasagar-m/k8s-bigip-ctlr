package controller

import (
	log "github.com/F5Networks/k8s-bigip-ctlr/v2/pkg/vlogger"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func (reqHandler *RequestHandler) checkPrimaryClusterHealthStatus() bool {

	status := false
	for i := 1; i <= 2; i++ {
		switch reqHandler.PrimaryClusterHealthProbeParams.EndPointType {
		case "http":
			status = reqHandler.getPrimaryClusterHealthStatusFromHTTPEndPoint()
		case "tcp":
			status = reqHandler.getPrimaryClusterHealthStatusFromTCPEndPoint()
		case "https":
			status = reqHandler.getPrimaryClusterHealthStatusFromHTTPSEndPoint()
		case "", "default":
			log.Debugf("[MultiCluster] unsupported primaryEndPoint specified under highAvailabilityCIS section: %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
			return false
		}

		if status {
			return status
		}
		time.Sleep(time.Duration(reqHandler.PrimaryClusterHealthProbeParams.retryInterval) * time.Second)
	}
	return false
}

// getPrimaryClusterHealthCheckEndPointType method determines type of probe to be done from CIS parameters
// http/tcp are the supported types
// when cis runs in primary mode this method should never be called
// should be called only when cis is running in secondary mode
func (reqHandler *RequestHandler) setPrimaryClusterHealthCheckEndPointType() {
	if reqHandler.PrimaryClusterHealthProbeParams.EndPoint != "" {
		if strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "tcp://") {
			reqHandler.PrimaryClusterHealthProbeParams.EndPointType = "tcp"
		} else if strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "https://") {
			reqHandler.PrimaryClusterHealthProbeParams.EndPointType = "https"
		} else if strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "http://") {
			reqHandler.PrimaryClusterHealthProbeParams.EndPointType = "http"
		} else {
			log.Warningf("[MultiCluster] unsupported primaryEndPoint protocol type configured under highAvailabilityCIS section. EndPoint: %v \n "+
				"supported protocols:[https, http, tcp] ", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
			os.Exit(1)
		}
	}
}

// getPrimaryClusterHealthStatusFromHTTPEndPoint check the primary cluster health using http endPoint
func (reqHandler *RequestHandler) getPrimaryClusterHealthStatusFromHTTPEndPoint() bool {

	if reqHandler.PrimaryClusterHealthProbeParams.EndPoint == "" {
		return false
	}
	if !strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "http://") {
		log.Debugf("[MultiCluster] Error: invalid primaryEndPoint detected under highAvailabilityCIS section: %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
		return false
	}

	req, err := http.NewRequest("GET", reqHandler.PrimaryClusterHealthProbeParams.EndPoint, nil)
	if err != nil {
		log.Errorf("[MultiCluster] Creating new HTTP request error: %v ", err)
		return false
	}

	timeOut := reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout
	defer func() {
		reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout = timeOut
	}()
	if reqHandler.PrimaryClusterHealthProbeParams.statusChanged {
		log.Debugf("[MultiCluster] posting GET Check primaryEndPoint Health request on %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
	}
	reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout = 10 * time.Second

	httpResp := reqHandler.httpGetReq(req)
	if httpResp == nil {
		return false
	}
	switch httpResp.StatusCode {
	case http.StatusOK:
		return true
	case http.StatusNotFound, http.StatusInternalServerError:
		log.Debugf("[MultiCluster] error fetching primaryEndPoint health status. endPoint:%v, statusCode: %v, error:%v",
			reqHandler.PrimaryClusterHealthProbeParams.EndPoint, httpResp.StatusCode, httpResp.Request.Response)
	}
	return false
}

// getPrimaryClusterHealthStatusFromHTTPSEndPoint check the primary cluster health using https endPoint
func (reqHandler *RequestHandler) getPrimaryClusterHealthStatusFromHTTPSEndPoint() bool {

	if reqHandler.PrimaryClusterHealthProbeParams.EndPoint == "" {
		return false
	}
	if !strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "https://") {
		log.Debugf("[MultiCluster] Error: invalid primaryEndPoint detected under highAvailabilityCIS section: %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
		return false
	}

	req, err := http.NewRequest("GET", reqHandler.PrimaryClusterHealthProbeParams.EndPoint, nil)
	if err != nil {
		log.Errorf("[MultiCluster] Creating new HTTP request error: %v ", err)
		return false
	}

	timeOut := reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout
	defer func() {
		reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout = timeOut
	}()
	if reqHandler.PrimaryClusterHealthProbeParams.statusChanged {
		log.Debugf("[MultiCluster] posting GET Check primaryEndPoint Health request on %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
	}
	reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Timeout = 10 * time.Second

	httpResp := reqHandler.httpGetReq(req)
	if httpResp == nil {
		return false
	}
	switch httpResp.StatusCode {
	case http.StatusOK:
		return true
	case http.StatusNotFound, http.StatusInternalServerError:
		log.Debugf("[MultiCluster] error fetching primaryEndPoint health status. endPoint:%v, statusCode: %v, error:%v",
			reqHandler.PrimaryClusterHealthProbeParams.EndPoint, httpResp.StatusCode, httpResp.Request.Response)
	}
	return false
}

// getPrimaryClusterHealthStatusFromTCPEndPoint check the primary cluster health using tcp endPoint
func (reqHandler *RequestHandler) getPrimaryClusterHealthStatusFromTCPEndPoint() bool {
	if reqHandler.PrimaryClusterHealthProbeParams.EndPoint == "" {
		return false
	}
	if !strings.HasPrefix(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "tcp://") {
		log.Debugf("[MultiCluster] invalid primaryEndPoint health probe tcp endpoint: %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint)
		return false
	}

	_, err := net.Dial("tcp", strings.TrimLeft(reqHandler.PrimaryClusterHealthProbeParams.EndPoint, "tcp://"))
	if err != nil {
		log.Debugf("[MultiCluster] error connecting to primaryEndPoint tcp health probe: %v, error: %v", reqHandler.PrimaryClusterHealthProbeParams.EndPoint, err)
		return false
	}
	return true
}

func (reqHandler *RequestHandler) httpGetReq(request *http.Request) *http.Response {
	httpResp, err := reqHandler.PrimaryBigIPWorker.getPostManager().httpClient.Do(request)

	if err != nil {
		if reqHandler.PrimaryClusterHealthProbeParams.statusChanged {
			log.Debugf("[MultiCluster] REST call error: %v ", err)
		}
		return nil
	}

	return httpResp
}

/*
	* probePrimaryClusterHealthStatus runs as a thread
	* this method check the cluster health periodically
		* will start probing only after init state is processed
		* if cluster is up earlier and now its down then resource queue event will be triggered
		* if cluster is down earlier and now also its down then we will skip processing
		* if cluster is up and running there is no status change then we skip the processing

*/

func (ctlr *Controller) probePrimaryClusterHealthStatus() {
	for {
		if ctlr.initState {
			continue
		}
		ctlr.getPrimaryClusterHealthStatus()
	}
}

func (ctlr *Controller) getPrimaryClusterHealthStatus() {

	// only process when the cis is initialized
	status := ctlr.RequestHandler.checkPrimaryClusterHealthStatus()
	// if status is changed i.e from up -> down / down -> up
	ctlr.RequestHandler.PrimaryClusterHealthProbeParams.paramLock.Lock()
	if ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning != status {
		ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusChanged = true
		// if primary cis id down then post the config
		if !status {
			ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning = false
			ctlr.enqueuePrimaryClusterProbeEvent()
		} else {
			log.Infof("[MultiCluster] Primary CIS is active and secondary CIS is moving to inactive state")
			ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning = true
		}
		//update cccl global section with primary cluster running status
		doneCh, errCh, err := ctlr.RequestHandler.PrimaryBigIPWorker.ConfigWriter.SendSection("primary-cluster-status", ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning)

		if nil != err {
			log.Warningf("[MultiCluster] Failed to write primary-cluster-status section: %v", err)
		} else {
			select {
			case <-doneCh:
				log.Debugf("[MultiCluster] Wrote primary-cluster-status as %v", ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning)
			case e := <-errCh:
				log.Warningf("[MultiCluster] Failed to write primary-cluster-status config section: %v", e)
			case <-time.After(time.Second):
				log.Warningf("[MultiCluster] Did not receive write response in 1s")
			}
		}
	} else {
		ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusChanged = false
	}
	ctlr.RequestHandler.PrimaryClusterHealthProbeParams.paramLock.Unlock()
	// wait for configured probeInterval
	time.Sleep(time.Duration(ctlr.RequestHandler.PrimaryClusterHealthProbeParams.probeInterval) * time.Second)
}

func (ctlr *Controller) firstPollPrimaryClusterHealthStatus() {
	ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusRunning = ctlr.RequestHandler.checkPrimaryClusterHealthStatus()
	ctlr.RequestHandler.PrimaryClusterHealthProbeParams.statusChanged = true
}
