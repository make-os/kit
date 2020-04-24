package issues

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Issue", func() {
	var err error
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MakeIssueBody", func() {
		It("case 1 - only title is set", func() {
			str := MakeIssueBody("my title", "", 0, nil, nil, nil)
			Expect(str).To(Equal("---\ntitle: my title\n---\n"))
		})

		It("case 2 - only title,body are set", func() {
			str := MakeIssueBody("my title", "my body", 0, nil, nil, nil)
			Expect(str).To(Equal("---\ntitle: my title\n---\nmy body"))
		})

		It("case 3 - only title,body,replyTo are set", func() {
			str := MakeIssueBody("my title", "my body", 1, nil, nil, nil)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: 1\n---\nmy body"))
		})

		It("case 4 - only title,body,replyTo,labels are set", func() {
			str := MakeIssueBody("my title", "my body", 1, []string{"a", "b"}, nil, nil)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: 1\nlabels: [\"a\",\"b\"]\n---\nmy body"))
		})

		It("case 5 - only title,body,replyTo,labels,assignees are set", func() {
			str := MakeIssueBody("my title", "my body", 1, []string{"a", "b"}, []string{"a", "b"}, nil)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: 1\nlabels: [\"a\",\"b\"]\nassignees: [\"a\",\"b\"]\n---\nmy body"))
		})

		It("case 6 - only title,body,replyTo,labels,assignees,fixers are set", func() {
			str := MakeIssueBody("my title", "my body", 1, []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"})
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: 1\nlabels: [\"a\",\"b\"]\nassignees: [\"a\",\"b\"]\nfixers: [\"a\",\"b\"]\n---\nmy body"))
		})
	})
})
