package policy

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/remote/types"
	"gitlab.com/makeos/lobe/testutil"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/state"
)

func testCheckTxDetail(err error) func(params *types.TxDetail, keepers core.Keepers, index int) error {
	return func(params *types.TxDetail, keepers core.Keepers, index int) error { return err }
}

var _ = Describe("Auth", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		key = crypto.NewKeyFromIntSeed(1)
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MakePusherPolicyGroups", func() {
		var polGroups [][]*state.Policy
		var repoPolicy *state.Policy
		var namespacePolicy *state.ContributorPolicy
		var contribPolicy *state.ContributorPolicy
		var targetPusherAddr string

		BeforeEach(func() {
			targetPusherAddr = key.PushAddr().String()
		})

		When("repo config, repo namespace and repo contributor entry has policies", func() {
			BeforeEach(func() {

				// Add target pusher repo config policies
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: targetPusherAddr, Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)

				// Add target pusher namespace policies
				namespacePolicy = &state.ContributorPolicy{Object: "refs/heads/about", Action: "write"}
				ns := &state.Namespace{Contributors: map[string]*state.BaseContributor{
					key.PushAddr().String(): {Policies: []*state.ContributorPolicy{namespacePolicy}},
				}}

				// Add target pusher address repo contributor policies
				contribPolicy = &state.ContributorPolicy{Object: "refs/heads/dev", Action: "delete"}
				repoState.Contributors[key.PushAddr().String()] = &state.RepoContributor{
					Policies: []*state.ContributorPolicy{contribPolicy},
				}

				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, ns)
			})

			Specify("that each policy group is not empty", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[0]).To(HaveLen(1))
				Expect(polGroups[1]).To(HaveLen(1))
				Expect(polGroups[2]).To(HaveLen(1))
			})

			Specify("that index 0 includes pusher's repo contributor policy", func() {
				Expect(polGroups[0]).To(ContainElement(&state.Policy{
					Object:  "refs/heads/dev",
					Action:  "delete",
					Subject: key.PushAddr().String(),
				}))
			})

			Specify("that index 1 includes the pusher's namespace contributor policy", func() {
				Expect(polGroups[1]).To(ContainElement(&state.Policy{
					Object:  "refs/heads/about",
					Action:  "write",
					Subject: key.PushAddr().String(),
				}))
			})

			Specify("that index 1 includes the pusher's repo config policy", func() {
				Expect(polGroups[2]).To(ContainElement(&state.Policy{
					Object:  "refs/heads/master",
					Action:  "write",
					Subject: key.PushAddr().String(),
				}))
			})
		})

		When("repo config policies include a policy whose subject is not a push key ID or 'all' or 'contrib'", func() {
			BeforeEach(func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "some_subject", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
			})

			It("should not include the policy", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(0))
			})
		})

		When("repo config policies include a policy whose subject is 'all' or 'contrib' or a push key", func() {
			It("should return policy if its subject is 'all'", func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "all", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(1))
			})

			It("should return policy if its subject is 'contrib'", func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "contrib", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(1))
			})

			It("should return policy if its subject is 'creator'", func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "creator", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(1))
			})

			It("should return policy if its subject is a push key address", func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: key.PushAddr().String(), Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(1))
			})
		})

		When("repo config policies include a policy whose object is not a recognized reference name", func() {
			BeforeEach(func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "all", Object: "master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
			})

			It("should not include the policy", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(0))
			})
		})
	})

	Describe(".CheckPolicy", func() {
		var allowAction = "write"
		var denyAction = "deny-" + allowAction
		var enforcer EnforcerFunc
		var pushAddrA string

		BeforeEach(func() {
			pushAddrA = key.PushAddr().String()
		})

		It("should return error when reference type is unknown", func() {
			enforcer := GetPolicyEnforcer([][]*state.Policy{{{Object: "obj", Subject: "sub", Action: "ac"}}})
			err := CheckPolicy(enforcer, "refs/unknown/xyz", false, key.PushAddr().String(), false, "write")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unknown reference (refs/unknown/xyz)"))
		})

		When("action is allowed on any level", func() {
			It("should return nil at level 0", func() {
				policies := [][]*state.Policy{{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
			It("should return nil at level 1", func() {
				policies := [][]*state.Policy{{}, {{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
			It("should return nil at level 2", func() {
				policies := [][]*state.Policy{{}, {}, {{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("action does not have a policy", func() {
			It("should return err", func() {
				policies := [][]*state.Policy{{}, {}, {}}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is allowed on level 0 and denied on level 0", func() {
			It("should return err", func() {
				policies := [][]*state.Policy{
					{
						{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction},
						{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction},
					},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is allowed on level 0 and denied on level 1", func() {
			It("should return err", func() {
				policies := [][]*state.Policy{
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("action is denied on level 0 and allowed on level 1", func() {
			It("should return err", func() {
				policies := [][]*state.Policy{
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction}},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is denied on level 1 and allowed on level 2", func() {
			It("should return err", func() {
				policies := [][]*state.Policy{
					{},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction}},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is allowed for subject:'all' on level 2", func() {
			It("should return nil", func() {
				policies := [][]*state.Policy{
					{}, {},
					{{Subject: "all", Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("action is denied for subject:'all' on level 2", func() {
			It("should return error", func() {
				policies := [][]*state.Policy{
					{}, {},
					{{Subject: "all", Object: "refs/heads/master", Action: denyAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is denied for subject:'all' on level 2 and allowed at level 1", func() {
			It("should return nil", func() {
				policies := [][]*state.Policy{
					{},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
					{{Subject: "all", Object: "refs/heads/master", Action: denyAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("action is denied for subject:'pushAddrA' on level 2 and allowed for subject:all level 2", func() {
			It("should not authorize pushAddrA by returning error", func() {
				policies := [][]*state.Policy{
					{}, {},
					{
						{Subject: "all", Object: "refs/heads/master", Action: allowAction},
						{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction},
					},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is denied for subject:'all' on level 1 and allowed for subject:'pushAddrA' level 2", func() {
			It("should not authorize pushAddrA by returning error", func() {
				policies := [][]*state.Policy{
					{},
					{{Subject: "all", Object: "refs/heads/master", Action: denyAction}},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is denied for subject:'all' on level 1 and allowed for subject:'pushAddrA' level 0", func() {
			It("should return nil", func() {
				policies := [][]*state.Policy{
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
					{{Subject: "all", Object: "refs/heads/master", Action: denyAction}},
					{},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("action is denied on dir:refs/heads as subject:'all' on level 0 and allowed on refs/heads/master on level 1", func() {
			It("should not authorize pushAddrA by returning error", func() {
				policies := [][]*state.Policy{
					{{Subject: pushAddrA, Object: "refs/heads", Action: denyAction}},
					{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
			})
		})

		When("action is denied on dir:refs/heads as subject:'all' on level 0 and "+
			"dir:refs/tags as subject is allowed on level 0 and "+
			"query subject is refs/tags/tag1", func() {
			It("should return nil", func() {
				policies := [][]*state.Policy{
					{
						{Subject: "all", Object: "refs/heads", Action: denyAction},
						{Subject: pushAddrA, Object: "refs/tags", Action: allowAction},
					}, {}, {},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/tags/tag1", false, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("pusher is a contributor", func() {
			It("should return nil when action is allowed for subject:contrib, object:refs/heads/master", func() {
				policies := [][]*state.Policy{
					{{Subject: "contrib", Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, true, allowAction)
				Expect(err).To(BeNil())
			})

			It("should return nil when action is allowed for subject:contrib, object:refs/heads", func() {
				policies := [][]*state.Policy{
					{{Subject: "contrib", Object: "refs/heads", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, true, allowAction)
				Expect(err).To(BeNil())
			})
		})

		When("pusher is not a contributor", func() {
			It("should return error when action is not allowed", func() {
				policies := [][]*state.Policy{
					{{Subject: "contrib", Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
				Expect(err).ToNot(BeNil())
			})
		})

		When("pusher is the reference creator", func() {
			It("should return nil when action is allowed for subject:creator, object:refs/heads/master", func() {
				policies := [][]*state.Policy{
					{{Subject: "creator", Object: "refs/heads/master", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", true, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})

			It("should return nil when action is allowed for subject:creator, object:refs/heads", func() {
				policies := [][]*state.Policy{
					{{Subject: "creator", Object: "refs/heads", Action: allowAction}},
				}
				enforcer = GetPolicyEnforcer(policies)
				err = CheckPolicy(enforcer, "refs/heads/master", true, pushAddrA, false, allowAction)
				Expect(err).To(BeNil())
			})
		})

		Context("check goto in unnamed enforce() method within CheckPolicy", func() {
			When("reference=(refs/heads) is a root reference", func() {
				It("should skip to root reference check "+
					"(enforcer call count must be 4 (2 for sub:all, 2 for sub:pushKeyID, object=refs/heads/2)", func() {
					count := 0
					enforcer = func(subject, object, action string) (bool, int) {
						count++
						Expect(object).To(Equal("refs/heads"))
						return true, 0
					}
					err = CheckPolicy(enforcer, "refs/heads", false, pushAddrA, false, allowAction)
					Expect(count).To(Equal(4))
				})
			})
		})
	})
})
