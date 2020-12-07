package agent

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/make-os/kit/pkgs/cache"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Suite")
}

var _ = Describe("Agent", func() {
	BeforeEach(func() {
		mem = cache.NewCacheWithExpiringEntry(100, 1*time.Millisecond)
	})

	Describe(".setHandler", func() {
		It("should add key and passphrase", func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("", "/set", strings.NewReader(""))
			q := req.URL.Query()
			q.Add("key", "mykey")
			q.Add("pass", "mypass")
			req.URL.RawQuery = q.Encode()
			setHandler(rec, req)
			Expect(rec.Code).To(Equal(200))
			Expect(mem.Len()).To(Equal(1))
			Expect(mem.Has("mykey")).To(BeTrue())
			Expect(mem.Get("mykey")).To(Equal("mypass"))
		})

		When("ttl is set", func() {
			It("should remove key after ttl is exceeded", func() {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest("", "/set", strings.NewReader(""))
				q := req.URL.Query()
				q.Add("key", "mykey")
				q.Add("pass", "mypass")
				q.Add("ttl", (1 * time.Second).String())
				req.URL.RawQuery = q.Encode()
				setHandler(rec, req)
				Expect(rec.Code).To(Equal(200))
				Expect(mem.Len()).To(Equal(1))
				time.Sleep(1 * time.Second)
				Expect(mem.Len()).To(Equal(0))
			})
		})
	})

	Describe(".getHandler", func() {
		It("should get a key if it exists", func() {
			mem.Add("mykey", "mypass")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("", "/set", strings.NewReader(""))
			q := req.URL.Query()
			q.Add("key", "mykey")
			req.URL.RawQuery = q.Encode()
			getHandler(rec, req)

			Expect(rec.Body.String()).To(Equal("mypass"))
		})

		It("should return 404 status code when key does not exist", func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("", "/set", strings.NewReader(""))
			q := req.URL.Query()
			q.Add("key", "mykey")
			req.URL.RawQuery = q.Encode()
			getHandler(rec, req)
			Expect(rec.Code).To(Equal(404))
			Expect(rec.Body.String()).To(Equal(""))
		})
	})

	Describe(".RunAgentServer", func() {
		var svr *httptest.Server
		var port string

		BeforeEach(func() {
			svr = httptest.NewServer(getMux())
			url, _ := url.Parse(svr.URL)
			port = url.Port()
		})

		AfterEach(func() {
			svr.Close()
		})

		Describe(".SendSetRequest", func() {
			It("should set key and value", func() {
				err := SendSetRequest(port, "mykey", "value", 10)
				Expect(err).To(BeNil())
			})
		})

		Describe(".SendGetRequest", func() {
			It("should get key's value", func() {
				mem.Add("mykey", "some_value")
				val, err := SendGetRequest(port, "mykey")
				Expect(err).To(BeNil())
				Expect(val).To(Equal("some_value"))
			})
		})

		Describe(".IsAgentUp", func() {
			It("should set key and value", func() {
				Expect(IsAgentUp(port)).To(BeTrue())
			})
		})
	})
})
