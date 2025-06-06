package controller

import (
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backend Tests", func() {
	Describe("agent worker tests with different status codes", func() {
		var agent *Agent
		var postConfig *agentPostConfig
		var mockBaseAPIHandler *BaseAPIHandler
		BeforeEach(func() {
			mockBaseAPIHandler = newMockBaseAPIHandler()
			agent = &Agent{}
			tenantDeclMap := make(map[string]as3Tenant)
			tenantDeclMap["test"] = as3Tenant{
				"class":              "Tenant",
				"defaultRouteDomain": 0,
				as3SharedApplication: "shared",
				"label":              "cis2.x",
			}
			postConfig = &agentPostConfig{
				reqMeta: requestMeta{
					id: 1,
				},
				as3APIURL:             "https://127.0.0.1/mgmt/shared/appsvcs/declare",
				data:                  `{"class": "AS3", "declaration": {"class": "ADC", "test": {"class": "Tenant", "testApp": {"class": "Application", "webcert":{"class": "Certificate", "certificate": "abc", "privateKey": "abc", "chainCA": "abc"}}}}}`,
				incomingTenantDeclMap: tenantDeclMap,
				tenantResponseMap:     make(map[string]tenantResponse),
			}
		})
		It("Agent worker test", func() {
			client, _ := getMockHttpClient([]responseCtx{{
				tenant: "test",
				status: http.StatusOK,
				body:   io.NopCloser(strings.NewReader("{\"results\": [{\"code\": 200, \"message\": \"success\", \"tenant\": \"test\"}], \"declaration\": {\"class\": \"ADC\", \"test\": {\"class\": \"Tenant\", \"testApp\": {\"class\": \"Application\", \"webcert\":{\"class\": \"Certificate\", \"certificate\": \"abc\", \"privateKey\": \"abc\", \"chainCA\": \"abc\"}}}}}")),
			}}, http.MethodPost)
			mockBaseAPIHandler.httpClient = client
			agent.APIHandler = &APIHandler{
				LTM: &LTMAPIHandler{
					BaseAPIHandler: mockBaseAPIHandler,
				},
				GTM: &GTMAPIHandler{
					BaseAPIHandler: mockBaseAPIHandler,
				},
			}
			go agent.agentWorker()
			agent.LTM.PostManager.postChan <- postConfig
			response := <-agent.LTM.PostManager.respChan

			Expect(response).NotTo(BeNil(), "response should not be nil")
			Expect(response.tenantResponseMap["test"].agentResponseCode).To(Equal(http.StatusOK), "response code should be 200")

			close(agent.LTM.PostManager.postChan)
			close(agent.LTM.PostManager.respChan)
		})
	})
})
