package cache_test

import (
	"context"
	"net/http"

	"github.com/bborbe/stream/cache"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Cache", func() {
	var roundTripper http.RoundTripper
	var server *ghttp.Server
	BeforeEach(func() {
		roundTripper = cache.NewRoundTrip(context.Background(), http.DefaultClient)
		server = ghttp.NewServer()
		server.RouteToHandler(http.MethodGet, "/", func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
		})
	})
	AfterEach(func() {
		server.Close()
	})
	It("Compiles", func() {
		request, err := http.NewRequest(http.MethodGet, server.URL(), nil)
		Expect(err).To(BeNil())
		response, err := roundTripper.RoundTrip(request)
		Expect(err).To(BeNil())
		Expect(response.StatusCode).To(Equal(http.StatusOK))
	})
})
