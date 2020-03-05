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
				r.References = map[string]interface{}{
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
				r.Proposals = map[string]interface{}{"1": &RepoProposal{
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
			r.References = map[string]interface{}{"refs/heads/master": &Reference{}}
			Expect(r.IsNil()).To(BeFalse())
		})
	})

	Describe("References", func() {
		Describe(".Get", func() {
			It("should return bare reference when not found", func() {
				refs := References(map[string]interface{}{
					"refs/heads/master": &Reference{Nonce: 10},
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(BareReference()))
			})

			It("should return ref when found", func() {
				ref := &Reference{Nonce: 10}
				refs := References(map[string]interface{}{
					"refs/heads/dev": ref,
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(ref))
			})
		})

		Describe(".Has", func() {
			When("reference does not exist", func() {
				It("should return false", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]interface{}{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/master")).To(BeFalse())
				})
			})

			When("reference exist", func() {
				It("should return true", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]interface{}{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/dev")).To(BeTrue())
				})
			})
		})
	})

	Describe("RepoOwners", func() {
		var v RepoOwners

		BeforeEach(func() {
			v = RepoOwners(map[string]interface{}{
				"abc": &RepoOwner{JoinedAt: 100},
				"xyz": map[string]interface{}{
					"joinAt": 200,
				},
			})
		})

		Describe(".Get", func() {
			It("should return nil when key is not found", func() {
				Expect(v.Get("aaa")).To(BeNil())
			})

			It("should return RepoOwner when key is found", func() {
				Expect(v.Get("abc")).ToNot(BeNil())
				Expect(v.Get("abc")).To(BeAssignableToTypeOf(&RepoOwner{}))
				Expect(v.Get("xyz")).To(BeAssignableToTypeOf(&RepoOwner{}))
			})
		})

		Describe(".Has", func() {
			It("should return false when key is not found", func() {
				Expect(v.Has("aaa")).To(BeFalse())
			})

			It("should return true when key is found", func() {
				Expect(v.Has("xyz")).To(BeTrue())
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

	Describe("RepoConfig.MergeMap", func() {
		Context("merge all key/value in map", func() {
			base := &RepoConfig{
				Governance: &RepoConfigGovernance{
					ProposalProposee:                 1,
					ProposalProposeeLimitToCurHeight: true,
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
	})

	Describe("RepoConfig.Clone", func() {
		base := &RepoConfig{
			Governance: &RepoConfigGovernance{
				ProposalProposee:                 1,
				ProposalProposeeLimitToCurHeight: true,
			},
		}

		It("should clone into a different RepoConfig object", func() {
			clone := base.Clone()
			Expect(base).To(Equal(clone))
			Expect(fmt.Sprintf("%p", base)).ToNot(Equal(fmt.Sprintf("%p", clone)))
			Expect(fmt.Sprintf("%p", base.Governance)).ToNot(Equal(fmt.Sprintf("%p", clone.Governance)))
		})
	})

	Describe("RepoConfig.Merge", func() {
		When("other object is nil", func() {
			It("should change nothing", func() {
				o := &RepoConfig{Governance: &RepoConfigGovernance{ProposalProposee: 1}}
				o.Merge(nil)
				Expect(int(o.Governance.ProposalProposee)).To(Equal(1))

				o = &RepoConfig{Governance: &RepoConfigGovernance{ProposalProposee: 1}}
				o.Merge(&RepoConfig{})
				Expect(int(o.Governance.ProposalProposee)).To(Equal(1))
			})
		})

		When("other object is not nil", func() {
			It("should change base fields to values of non-zero, non-equal fields", func() {
				o := &RepoConfig{
					Governance: &RepoConfigGovernance{
						ProposalProposee:                 1,
						ProposalDur:                      2,
						ProposalTallyMethod:              4,
						ProposalThreshold:                10,
						ProposalQuorum:                   40,
						ProposalProposeeLimitToCurHeight: true,
						ProposalVetoQuorum:               10,
						ProposalVetoOwnersQuorum:         3,
					},
				}

				o2 := &RepoConfig{
					Governance: &RepoConfigGovernance{
						ProposalProposee:                 3,
						ProposalDur:                      5,
						ProposalTallyMethod:              6,
						ProposalThreshold:                11,
						ProposalQuorum:                   42,
						ProposalProposeeLimitToCurHeight: false,
						ProposalVetoQuorum:               11,
						ProposalVetoOwnersQuorum:         33,
					},
				}

				o.Merge(o2)
				Expect(o).To(Equal(o2))
			})
		})

		When("other object is not nil but some values are zero", func() {
			It("should change base fields to values of non-zero, non-equal fields", func() {
				o := &RepoConfig{
					Governance: &RepoConfigGovernance{
						ProposalProposee:                 1,
						ProposalDur:                      2,
						ProposalTallyMethod:              4,
						ProposalThreshold:                10,
						ProposalQuorum:                   40,
						ProposalProposeeLimitToCurHeight: true,
						ProposalVetoQuorum:               10,
						ProposalVetoOwnersQuorum:         3,
					},
				}

				o2 := &RepoConfig{
					Governance: &RepoConfigGovernance{
						ProposalProposee:                 3,
						ProposalDur:                      5,
						ProposalTallyMethod:              6,
						ProposalThreshold:                0,
						ProposalQuorum:                   42,
						ProposalProposeeLimitToCurHeight: false,
						ProposalVetoQuorum:               0,
						ProposalVetoOwnersQuorum:         33,
					},
				}

				o.Merge(o2)
				Expect(o).ToNot(Equal(o2))
				Expect(o.Governance.ProposalThreshold).ToNot(BeZero())
				Expect(o.Governance.ProposalVetoQuorum).ToNot(BeZero())
			})
		})
	})
})
