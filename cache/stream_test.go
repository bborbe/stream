package cache_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/bborbe/stream/cache"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SharedReader", func() {
	It("contains input content", func() {
		input := bytes.NewBufferString("hello world")
		sharedReadCloser := cache.NewStream(context.Background(), ioutil.NopCloser(input))
		defer sharedReadCloser.Close()
		Expect(readAllToString(sharedReadCloser.ReadCloser())).To(Equal("hello world"))
	})
	It("return sample content for second reader", func() {
		input := bytes.NewBufferString("hello world")
		sharedReadCloser := cache.NewStream(context.Background(), ioutil.NopCloser(input))
		defer sharedReadCloser.Close()
		Expect(readAllToString(sharedReadCloser.ReadCloser())).To(Equal("hello world"))
		Expect(readAllToString(sharedReadCloser.ReadCloser())).To(Equal("hello world"))
	})
})

func readAllToString(reader io.ReadCloser) string {
	output := &bytes.Buffer{}
	io.Copy(output, reader)
	return output.String()
}
