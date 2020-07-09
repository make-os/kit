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
				config.Gov = &RepoConfigGovernance{PropDuration: 100}
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

	Describe("RepoConfig.MergeMap", func() {
		Context("Governance Merging", func() {
			base := &RepoConfig{
				Gov: &RepoConfigGovernance{
					Voter:              1,
					ReqVoterJoinHeight: true,
					PropQuorum:         1,
				},
				Policies: []*Policy{
					{Subject: "user1", Object: "dev", Action: "deny"},
				},
			}

			It("should update base object", func() {
				base.MergeMap(map[string]interface{}{
					"governance": map[string]interface{}{
						"propVoter":              13,
						"requireVoterJoinHeight": false,
						"propQuorum":             0,
					},
				})
				Expect(int(base.Gov.Voter)).To(Equal(13))
				Expect(base.Gov.ReqVoterJoinHeight).To(BeFalse())
				Expect(base.Gov.PropQuorum).To(BeZero())
			})
		})

		Context("Policies Merging", func() {
			var base *RepoConfig

			BeforeEach(func() {
				base = &RepoConfig{
					Gov: &RepoConfigGovernance{Voter: 1, PropDuration: 100, PropFee: 12},
					Policies: []*Policy{
						{Subject: "user1", Object: "dev", Action: "deny"},
					},
				}
			})

			It("should add to existing policy", func() {
				err := base.MergeMap(map[string]interface{}{
					"policies": []interface{}{
						map[string]interface{}{"sub": "sub2", "obj": "branch_dev", "act": "delete"},
					},
				})
				Expect(err).To(BeNil())
				Expect(base.Policies).To(HaveLen(2))
				Expect(base.Policies[0].Subject).To(Equal("user1"))
				Expect(base.Policies[0].Object).To(Equal("dev"))
				Expect(base.Policies[0].Action).To(Equal("deny"))
				Expect(base.Policies[1].Subject).To(Equal("sub2"))
				Expect(base.Policies[1].Object).To(Equal("branch_dev"))
				Expect(base.Policies[1].Action).To(Equal("delete"))
				Expect(base.Gov.Voter).To(Equal(VoterType(1)))
				Expect(base.Gov.PropDuration.UInt64()).To(Equal(uint64(100)))
				Expect(base.Gov.PropFee).To(Equal(float64(12)))
			})
		})
	})

	Describe("RepoConfig.Clone", func() {
		base := &RepoConfig{
			Gov: &RepoConfigGovernance{
				Voter:              1,
				ReqVoterJoinHeight: true,
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

	Describe("RepoConfig.FromMap", func() {
		var cfg1 = &RepoConfig{
			Gov: &RepoConfigGovernance{
				Voter:                1,
				PropCreator:          1,
				ReqVoterJoinHeight:   true,
				PropDuration:         12,
				PropFeeDepositDur:    122,
				PropTallyMethod:      1,
				PropQuorum:           25,
				PropThreshold:        40,
				PropVetoQuorum:       50,
				PropVetoOwnersQuorum: 30,
				PropFee:              10,
				PropFeeRefundType:    1,
				NoPropFeeForMergeReq: true,
			},
			Policies: []*Policy{
				{Subject: "sub", Object: "obj", Action: "act"},
			},
		}

		It("should populate from a map stripped of custom types", func() {
			m := cfg1.ToBasicMap()
			cfg2 := &RepoConfig{Gov: &RepoConfigGovernance{}, Policies: []*Policy{}}
			cfg2.FromMap(m)
			Expect(cfg1).To(Equal(cfg2))
		})
	})
})
