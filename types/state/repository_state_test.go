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
				r.References = map[string]*Reference{
					"refs/heads/master": &Reference{
						Nonce: 20,
					},
				}
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

		Context("Decode Proposals", func() {
			BeforeEach(func() {
				r = BareRepository()
				r.Proposals = map[string]*RepoProposal{"1": &RepoProposal{
					Creator: "address1",
				}}
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
				config.Governance = &RepoConfigGovernance{ProposalDur: 100}
				config.ACL = RepoACLPolicies{"obj": &RepoACLPolicy{"obj", "sub", "deny"}}
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
			r.References = map[string]*Reference{"refs/heads/master": &Reference{}}
			Expect(r.IsNil()).To(BeFalse())
		})
	})

	Describe("References", func() {
		Describe(".Get", func() {
			It("should return bare reference when not found", func() {
				refs := References(map[string]*Reference{
					"refs/heads/master": &Reference{Nonce: 10},
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
			v = RepoOwners(map[string]*RepoOwner{
				"abc": &RepoOwner{JoinedAt: 100},
				"xyz": &RepoOwner{JoinedAt: 200},
			})
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
				var owners = []string{}
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
						ProposalProposee:                 1,
						ProposalProposeeLimitToCurHeight: true,
					},
					ACL: RepoACLPolicies{
						"user1_dev": &RepoACLPolicy{Subject: "user1", Object: "dev", Action: "deny"},
					},
				}

				It("should update base object", func() {
					base.MergeMap(map[string]interface{}{
						"name": "some-name",
						"gov": map[string]interface{}{
							"propProposee":                 13,
							"propProposeeLimitToCurHeight": false,
						},
					})
					Expect(int(base.Governance.ProposalProposee)).To(Equal(13))
					Expect(base.Governance.ProposalProposeeLimitToCurHeight).To(BeFalse())
				})
			})

			Context("ACL merge", func() {
				When("ACL includes one policy named `user1_dev`", func() {
					base := &RepoConfig{
						ACL: RepoACLPolicies{
							"user1_dev": &RepoACLPolicy{Subject: "user1", Object: "dev", Action: "deny"},
						},
					}

					It("should replace existing policy if their name matches", func() {
						base.MergeMap(map[string]interface{}{
							"acl": map[string]interface{}{
								"user1_dev": map[string]interface{}{"sub": "sub2", "obj": "branch_dev", "act": "delete"},
							},
						})
						Expect(base.ACL).To(HaveLen(1))
						Expect(base.ACL).To(HaveKey("user1_dev"))
						Expect(base.ACL["user1_dev"].Subject).To(Equal("sub2"))
						Expect(base.ACL["user1_dev"].Object).To(Equal("branch_dev"))
						Expect(base.ACL["user1_dev"].Action).To(Equal("delete"))
					})
				})

				When("ACL includes one policy named `user1_dev`", func() {
					base := &RepoConfig{
						ACL: RepoACLPolicies{
							"user1_dev": &RepoACLPolicy{Subject: "user1", Object: "dev", Action: "deny"},
						},
					}

					It("should add policy if it does not already exist", func() {
						base.MergeMap(map[string]interface{}{
							"acl": map[string]interface{}{
								"user2_dev": map[string]interface{}{"sub": "sub2", "obj": "branch_dev", "act": "delete"},
							},
						})
						Expect(base.ACL).To(HaveLen(2))
						Expect(base.ACL).To(HaveKey("user1_dev"))
						Expect(base.ACL).To(HaveKey("user2_dev"))
						Expect(base.ACL["user2_dev"].Subject).To(Equal("sub2"))
						Expect(base.ACL["user2_dev"].Object).To(Equal("branch_dev"))
						Expect(base.ACL["user2_dev"].Action).To(Equal("delete"))
					})
				})
			})
		})

		Describe(".Clone", func() {
			base := &RepoConfig{
				Governance: &RepoConfigGovernance{
					ProposalProposee:                 1,
					ProposalProposeeLimitToCurHeight: true,
				},
				ACL: map[string]*RepoACLPolicy{},
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
