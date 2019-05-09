package cache_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"

	"github.com/bborbe/stream/cache"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	It("returns response", func() {
		inputResponse := &http.Response{}
		inputResponse.Body = ioutil.NopCloser(&bytes.Buffer{})
		connection := cache.NewConnection(context.Background(), inputResponse)
		outputResponse := connection.Response()
		Expect(outputResponse).NotTo(BeNil())
	})
	It("returns new response", func() {
		inputResponse := &http.Response{}
		inputResponse.Body = ioutil.NopCloser(&bytes.Buffer{})
		connection := cache.NewConnection(context.Background(), inputResponse)
		outputResponse := connection.Response()
		Expect(outputResponse).NotTo(BeNil())
		Expect(inputResponse).NotTo(Equal(outputResponse))
	})
})
