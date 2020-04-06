package rest

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("", func() {
	Describe(".V1Path", func() {
		When("namespace='my_namespace' and method='my_method'", func() {
			It("should return /v1/my_namespace/my_method", func() {
				path := V1Path("my_namespace", "my_method")
				Expect(path).To(Equal("/v1/my_namespace/my_method"))
			})
		})
	})
})
