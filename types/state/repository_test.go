package state

import (
	"fmt"

	"github.com/AlekSi/pointer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository", func() {
	Describe(".NewRepositoryFromBytes", func() {
		var r *Repository
		var expectedBz []byte

		Context("with malformed byte slice", func() {
			It("should return err", func() {
				_, err := NewRepositoryFromBytes([]byte{1, 2, 3})
				Expect(err).ToNot(BeNil())
			})
		})

		Context("Decode References", func() {
			BeforeEach(func() {
				r = BareRepository()
				r.References = map[string]*Reference{"refs/heads/master": {Nonce: 20}}
				expectedBz = r.Bytes()
			})

			It("should return bytes", func() {
				Expect(expectedBz).ToNot(BeEmpty())
			})

			It("should return object", func() {
				res, err := NewRepositoryFromBytes(expectedBz)
				Expect(err).To(BeNil())
				Expect(res.References).To(HaveKey("refs/heads/master"))
				Expect(res.References.Get("refs/heads/master").Nonce.UInt64()).To(Equal(uint64(20)))
			})
		})

		Context("Decode Proposals", func() {
			BeforeEach(func() {
				r = BareRepository()
				prop := BareRepoProposal()
				prop.Creator = "address1"
				r.Proposals = map[string]*RepoProposal{"1": prop}
				expectedBz = r.Bytes()
			})

			It("should return bytes", func() {
				Expect(expectedBz).ToNot(BeEmpty())
			})

			It("should return object", func() {
				res, err := NewRepositoryFromBytes(expectedBz)
				Expect(err).To(BeNil())
				Expect(res.Bytes()).To(Equal(r.Bytes()))
			})
		})

		Context("Decode Owners", func() {
			BeforeEach(func() {
				r = BareRepository()
				r.AddOwner("owner_addr", &RepoOwner{Creator: true})
				expectedBz = r.Bytes()
			})

			It("should return bytes", func() {
				Expect(expectedBz).ToNot(BeEmpty())
			})

			It("should return object", func() {
				res, err := NewRepositoryFromBytes(expectedBz)
				Expect(err).To(BeNil())
				Expect(res.Bytes()).To(Equal(r.Bytes()))
			})
		})

		Context("Decode Config", func() {
			BeforeEach(func() {
				r = BareRepository()
				r.Balance = "100"
				config := BareRepoConfig()
				config.Gov = &RepoConfigGovernance{PropDuration: pointer.ToStringOrNil("100")}
				config.Policies = []*Policy{{"obj", "sub", "deny"}}
				r.Config = config
				expectedBz = r.Bytes()
			})

			It("should return bytes", func() {
				Expect(expectedBz).ToNot(BeEmpty())
			})

			It("should return object", func() {
				res, err := NewRepositoryFromBytes(expectedBz)
				Expect(err).To(BeNil())
				Expect(r.Bytes()).To(Equal(res.Bytes()))
			})
		})
	})

	Describe("BareRepository.IsEmpty", func() {
		It("should return true when no fields are set", func() {
			r := BareRepository()
			Expect(r.IsEmpty()).To(BeTrue())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.AddOwner("owner_addr", &RepoOwner{Creator: true})
			Expect(r.IsEmpty()).To(BeFalse())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.References = map[string]*Reference{"refs/heads/master": {}}
			Expect(r.IsEmpty()).To(BeFalse())
		})
	})

	Describe("References", func() {
		Describe(".Get", func() {
			It("should return bare reference when not found", func() {
				refs := References(map[string]*Reference{
					"refs/heads/master": {Nonce: 10},
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(BareReference()))
			})

			It("should return ref when found", func() {
				ref := &Reference{Nonce: 10}
				refs := References(map[string]*Reference{
					"refs/heads/dev": ref,
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(ref))
			})
		})

		Describe(".Has", func() {
			When("reference does not exist", func() {
				It("should return false", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]*Reference{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/master")).To(BeFalse())
				})
			})

			When("reference exist", func() {
				It("should return true", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]*Reference{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/dev")).To(BeTrue())
				})
			})
		})
	})

	Describe("RepoOwners", func() {
		var v RepoOwners

		BeforeEach(func() {
			v = map[string]*RepoOwner{
				"abc": {JoinedAt: 100},
				"xyz": {JoinedAt: 200},
			}
		})

		Describe(".Get", func() {
			It("should return nil when key is not found", func() {
				Expect(v.Get("aaa")).To(BeNil())
			})

			It("should return RepoOwner when key is found", func() {
				Expect(v.Get("abc")).ToNot(BeNil())
				Expect(v.Get("abc")).To(BeAssignableToTypeOf(&RepoOwner{}))
			})
		})

		Describe(".Has", func() {
			It("should return false when key is not found", func() {
				Expect(v.Has("aaa")).To(BeFalse())
			})

			It("should return true when key is found", func() {
				Expect(v.Has("abc")).To(BeTrue())
			})
		})

		Describe(".ForEach", func() {
			It("should pass all values", func() {
				var owners []string
				v.ForEach(func(o *RepoOwner, addr string) {
					owners = append(owners, addr)
				})
				Expect(owners).To(ContainElement("xyz"))
				Expect(owners).To(ContainElement("abc"))
			})
		})
	})

	Describe("RepoConfig.Clone", func() {
		base := &RepoConfig{
			Gov: &RepoConfigGovernance{
				Voter:       pointer.ToInt(1),
				UsePowerAge: pointer.ToBool(true),
			},
			Policies: []*Policy{},
		}

		It("should clone into a different RepoConfig object", func() {
			clone := base.Clone()
			Expect(base).To(Equal(clone))
			Expect(fmt.Sprintf("%p", base)).ToNot(Equal(fmt.Sprintf("%p", clone)))
			Expect(fmt.Sprintf("%p", base.Gov)).ToNot(Equal(fmt.Sprintf("%p", clone.Gov)))
		})
	})
})
