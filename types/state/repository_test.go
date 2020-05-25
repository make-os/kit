package state

import (
	"fmt"

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
				Expect(res.References.Get("refs/heads/master").Nonce).To(Equal(uint64(20)))
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
				Expect(res).To(Equal(r))
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
				Expect(res).To(Equal(r))
			})
		})

		Context("Decode Config", func() {
			BeforeEach(func() {
				r = BareRepository()
				r.Balance = "100"
				config := BareRepoConfig()
				config.Governance = &RepoConfigGovernance{ProposalDuration: 100}
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
				Expect(r).To(Equal(res))
			})
		})
	})

	Describe("BareRepository.IsNil", func() {
		It("should return true when no fields are set", func() {
			r := BareRepository()
			Expect(r.IsNil()).To(BeTrue())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.AddOwner("owner_addr", &RepoOwner{Creator: true})
			Expect(r.IsNil()).To(BeFalse())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.References = map[string]*Reference{"refs/heads/master": {}}
			Expect(r.IsNil()).To(BeFalse())
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

	Describe("RepoConfig", func() {
		Describe(".MergeMap", func() {
			Context("merge all key/value in map", func() {
				base := &RepoConfig{
					Governance: &RepoConfigGovernance{
						Voter:               1,
						VoterAgeAsCurHeight: true,
						ProposalQuorum:      1,
					},
					Policies: []*Policy{{Subject: "user1", Object: "dev", Action: "deny"}},
				}

				It("should update base object", func() {
					base.MergeMap(map[string]interface{}{
						"name": "some-name",
						"governance": map[string]interface{}{
							"propVoter":           13,
							"voterAgeAsCurHeight": false,
							"propQuorum":          0,
						},
					})
					Expect(int(base.Governance.Voter)).To(Equal(13))
					Expect(base.Governance.VoterAgeAsCurHeight).To(BeFalse())
				})
			})

			Context("Policies merge", func() {
				When("Policies includes one policy named `user1_dev`", func() {
					var base *RepoConfig

					BeforeEach(func() {
						base = &RepoConfig{
							Governance: &RepoConfigGovernance{Voter: 1, ProposalDuration: 100, ProposalFee: 12},
							Policies:   []*Policy{{Subject: "user1", Object: "dev", Action: "deny"}},
						}
					})

					It("should replace existing policy only", func() {
						base.MergeMap(map[string]interface{}{
							"policies": []map[string]interface{}{{"sub": "sub2", "obj": "branch_dev", "act": "delete"}},
						})
						Expect(base.Policies).To(HaveLen(1))
						Expect(base.Policies[0].Subject).To(Equal("sub2"))
						Expect(base.Policies[0].Object).To(Equal("branch_dev"))
						Expect(base.Policies[0].Action).To(Equal("delete"))
						Expect(base.Governance.Voter).To(Equal(VoterType(1)))
						Expect(base.Governance.ProposalDuration).To(Equal(uint64(100)))
					})

					It("should replace governance config only", func() {
						base.MergeMap(map[string]interface{}{
							"governance": map[string]interface{}{"propVoter": 2, "propDur": 10},
						})
						Expect(base.Policies).To(HaveLen(1))
						Expect(base.Policies[0].Subject).To(Equal("user1"))
						Expect(base.Policies[0].Object).To(Equal("dev"))
						Expect(base.Policies[0].Action).To(Equal("deny"))
						Expect(base.Governance.Voter).To(Equal(VoterType(2)))
						Expect(base.Governance.ProposalDuration).To(Equal(uint64(10)))
						Expect(base.Governance.ProposalFee).To(Equal(float64(12)))
					})

				})

			})
		})

		Describe(".Clone", func() {
			base := &RepoConfig{
				Governance: &RepoConfigGovernance{
					Voter:               1,
					VoterAgeAsCurHeight: true,
				},
				Policies: []*Policy{},
			}

			It("should clone into a different RepoConfig object", func() {
				clone := base.Clone()
				Expect(base).To(Equal(clone))
				Expect(fmt.Sprintf("%p", base)).ToNot(Equal(fmt.Sprintf("%p", clone)))
				Expect(fmt.Sprintf("%p", base.Governance)).ToNot(Equal(fmt.Sprintf("%p", clone.Governance)))
			})
		})
	})
})
