package modules_test

import (
	"bytes"
	"io/ioutil"

	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("ConsoleUtilModule", func() {
	var m *modules.ConsoleUtilModule
	var out *bytes.Buffer
	var vm = otto.New()

	BeforeEach(func() {
		out = bytes.NewBuffer(nil)
		m = modules.NewConsoleUtilModule(out)
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceConsoleUtil)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".PrettyPrint", func() {
		BeforeEach(func() {
			m.ConfigureVM(vm)
		})

		It("should panic when unable to marshal input", func() {
			assert.Panics(GinkgoT(), func() { m.PrettyPrint(func() {}) })
		})

		It("should output pretty printed object", func() {
			m.PrettyPrint([]int{1, 2, 3})
			Expect(out.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".Dump", func() {
		It("should output dumped object to buffer", func() {
			m.Dump([]int{1, 2, 3})
			Expect(out.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".Diff", func() {
		It("should output diff result to buffer", func() {
			m.Diff(1, 2)
			Expect(out.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".Eval", func() {
		BeforeEach(func() {
			m.ConfigureVM(vm)
		})

		It("should panic when unable to evaluate source", func() {
			assert.Panics(GinkgoT(), func() { m.Eval("{") })
		})

		It("should panic when unable to evaluate source throw runtime error", func() {
			assert.Panics(GinkgoT(), func() { m.Eval(`throw new Error('Whoops!')`) })
		})

		It("should not panic and return expected evaluated result", func() {
			var res interface{}
			Expect(func() { res = m.Eval(`2+2`) }).ToNot(Panic())
			Expect(res.(otto.Value).String()).To(Equal("4"))
		})
	})

	Describe(".Eval", func() {
		BeforeEach(func() {
			m.ConfigureVM(vm)
		})

		It("should panic when unable to read source file", func() {
			assert.Panics(GinkgoT(), func() { m.EvalFile("unknown_file") })
		})

		It("should panic when unable to evaluate source file dues to runtime error", func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).To(BeNil())
			f.WriteString(`throw new Error('Whoops!')`)
			f.Close()
			assert.Panics(GinkgoT(), func() { m.EvalFile(f.Name()) })
		})

		It("should not panic and return expected evaluated result", func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).To(BeNil())
			f.WriteString(`2+2`)
			f.Close()
			var res interface{}
			Expect(func() { res = m.EvalFile(f.Name()) }).ToNot(Panic())
			Expect(res.(otto.Value).String()).To(Equal("4"))
		})
	})

	Describe(".ReadFile", func() {
		It("should panic when unable to read file", func() {
			assert.Panics(GinkgoT(), func() { m.ReadFile("unknown_file") })
		})

		It("should read file and return content", func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).To(BeNil())
			f.WriteString(`2+2`)
			f.Close()
			var res []byte
			Expect(func() { res = m.ReadFile(f.Name()) }).ToNot(Panic())
			Expect(string(res)).To(Equal(`2+2`))
		})
	})

	Describe(".ReadTextFile", func() {
		It("should panic when unable to read file", func() {
			assert.Panics(GinkgoT(), func() { m.ReadTextFile("unknown_file") })
		})

		It("should read file and return content", func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).To(BeNil())
			f.WriteString(`2+2`)
			f.Close()
			var res string
			Expect(func() { res = m.ReadTextFile(f.Name()) }).ToNot(Panic())
			Expect(res).To(Equal(`2+2`))
		})
	})

	Describe(".TreasuryAddress", func() {
		It("should return treasury address", func() {
			Expect(m.TreasuryAddress()).To(Equal(params.TreasuryAddress))
		})
	})

	Describe(".GenKey", func() {
		It("should generate key with seed", func() {
			res := m.GenKey(1)
			res2 := m.GenKey(1)
			Expect(res).To(Equal(util.Map{
				"address": "os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8",
				"pubkey":  "48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC",
				"privkey": "wU7ckbRBWevtkoT9QoET1adGCsABPRtyDx5T9EHZ4paP78EQ1w5sFM2sZg87fm1N2Np586c98GkYwywvtgy9d2gEpWbsbU",
			}))
			Expect(res).To(Equal(res2))
		})
	})
})
