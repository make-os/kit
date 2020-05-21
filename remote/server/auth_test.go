package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/mr-tron/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
)

func testCheckTxDetail(err error) func(params *types.TxDetail, keepers core.Keepers, index int) error {
	return func(params *types.TxDetail, keepers core.Keepers, index int) error { return err }
}

var _ = Describe("Auth", func() {
	var err error
	var cfg *config.AppConfig
	var repoName, path string
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var key, key2 *crypto.Key
	var svr *Server

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		key = crypto.NewKeyFromIntSeed(1)
		key2 = crypto.NewKeyFromIntSeed(2)

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		_, err = repo.Get(path)
		Expect(err).To(BeNil())

		ctrl = gomock.NewController(GinkgoT())
		mocksObjs := testutil.MockLogic(ctrl)
		mockLogic = mocksObjs.Logic

		mockDHTNode := mocks.NewMockDHTNode(ctrl)
		mockMempool := mocks.NewMockMempool(ctrl)
		mockBlockGetter := mocks.NewMockBlockGetter(ctrl)
		svr = NewManager(cfg, "127.0.0.1:0000", mockLogic, mockDHTNode, mockMempool, mockBlockGetter)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".authenticate", func() {
		When("there are two or more transaction details", func() {
			When("they are signed with different push keys", func() {
				BeforeEach(func() {
					txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
					txD2 := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key2.PushAddr().String()}
					repoState := state.BareRepository()
					repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
					txDetails := []*types.TxDetail{txD, txD2}
					_, err = authenticate(txDetails, repoState, state.BareNamespace(), mockLogic, testCheckTxDetail(nil))
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("index:1, field:pkID, msg:token siblings must be signed with the same push key"))
				})
			})
		})

		When("the details have different target repository name", func() {
			BeforeEach(func() {
				txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
				txD2 := &types.TxDetail{RepoName: "repo2", RepoNamespace: "ns1", Nonce: 2, PushKeyID: key.PushAddr().String()}
				txDetails := []*types.TxDetail{txD, txD2}
				repoState := state.BareRepository()
				repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
				_, err = authenticate(txDetails, repoState, state.BareNamespace(), mockLogic, testCheckTxDetail(nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:1, field:repo, msg:all push tokens must target the same repository"))
			})
		})

		When("the details have different nonce", func() {
			BeforeEach(func() {
				txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
				txD2 := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 2, PushKeyID: key.PushAddr().String()}
				txDetails := []*types.TxDetail{txD, txD2}
				repoState := state.BareRepository()
				repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
				_, err = authenticate(txDetails, repoState, state.BareNamespace(), mockLogic, testCheckTxDetail(nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:1, field:nonce, msg:all push tokens must have the same nonce"))
			})
		})

		When("the details have different target namespace", func() {
			BeforeEach(func() {
				txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
				txD2 := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns2", Nonce: 1, PushKeyID: key.PushAddr().String()}
				txDetails := []*types.TxDetail{txD, txD2}
				repoState := state.BareRepository()
				repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
				_, err = authenticate(txDetails, repoState, state.BareNamespace(), mockLogic, testCheckTxDetail(nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:1, field:namespace, msg:all push tokens must target the same namespace"))
			})
		})

		It("should return error when a reference transaction detail failed validation", func() {
			txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
			txDetails := []*types.TxDetail{txD}
			repoState := state.BareRepository()
			repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
			_, err := authenticate(txDetails, repoState, &state.Namespace{}, mockLogic, testCheckTxDetail(fmt.Errorf("bad error")))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("token error: bad error"))
		})

		Context("on success", func() {
			When("push key is a repo contributor", func() {
				BeforeEach(func() {
					txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
					txD2 := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
					txDetails := []*types.TxDetail{txD, txD2}
					repoState := state.BareRepository()
					repoState.Contributors = map[string]*state.RepoContributor{key.PushAddr().String(): {}}
					_, err = authenticate(txDetails, repoState, &state.Namespace{}, mockLogic, testCheckTxDetail(nil))
				})

				It("should return no error", func() {
					Expect(err).To(BeNil())
				})
			})

			When("push key is a namespace contributor", func() {
				BeforeEach(func() {
					txD := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
					txD2 := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1", Nonce: 1, PushKeyID: key.PushAddr().String()}
					txDetails := []*types.TxDetail{txD, txD2}
					ns := &state.Namespace{}
					ns.Contributors = map[string]*state.BaseContributor{key.PushAddr().String(): {}}
					_, err = authenticate(txDetails, state.BareRepository(), ns, mockLogic, testCheckTxDetail(nil))
				})

				It("should return no error", func() {
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".makePusherPolicyGroups", func() {
		var polGroups = [][]*state.Policy{}
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

				polGroups = policy.MakePusherPolicyGroups(key.PushAddr().String(), repoState, ns)
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

		When("repo config policies include a policy whose subject is not a push key ID or 'all'", func() {
			BeforeEach(func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "some_subject", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = policy.MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
			})

			It("should not include the policy", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(0))
			})
		})

		When("repo config policies include a policy whose subject is 'all'", func() {
			BeforeEach(func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "all", Object: "refs/heads/master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = policy.MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
			})

			It("should include the policy", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(1))
			})
		})

		When("repo config policies include a policy whose object is not a recognized reference name", func() {
			BeforeEach(func() {
				repoState := state.BareRepository()
				repoPolicy = &state.Policy{Subject: "all", Object: "master", Action: "write"}
				repoState.Config.Policies = append(repoState.Config.Policies, repoPolicy)
				polGroups = policy.MakePusherPolicyGroups(key.PushAddr().String(), repoState, state.BareNamespace())
			})

			It("should not include the policy", func() {
				Expect(polGroups).To(HaveLen(3))
				Expect(polGroups[2]).To(HaveLen(0))
			})
		})
	})

	Describe(".handleAuth", func() {
		var w http.ResponseWriter

		BeforeEach(func() {
			w = httptest.NewRecorder()
		})

		When("request method is GET", func() {
			It("should return nil transaction details, enforcer and error", func() {
				req := httptest.NewRequest("GET", "https://127.0.0.1", bytes.NewReader(nil))
				txDetails, enforcer, err := svr.handleAuth(req, w, &state.Repository{}, &state.Namespace{})
				Expect(err).To(BeNil())
				Expect(txDetails).To(BeNil())
				Expect(enforcer).To(BeNil())
			})
		})

		When("a push token is not provided", func() {
			It("should return error", func() {
				req := httptest.NewRequest("POST", "https://127.0.0.1", bytes.NewReader(nil))
				_, _, err := svr.handleAuth(req, w, &state.Repository{}, &state.Namespace{})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrPushTokenRequired))
			})
		})

		When("a push token is malformed", func() {
			It("should return error", func() {
				req := httptest.NewRequest("POST", "https://127.0.0.1", bytes.NewReader(nil))
				req.SetBasicAuth("xyz-malformed", "")
				_, _, err := svr.handleAuth(req, w, &state.Repository{}, &state.Namespace{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("malformed push token at index 0. Unable to decode"))
			})
		})

		When("a push token is ok but failed authentication (and validation)", func() {
			It("should return error", func() {
				txDetail := &types.TxDetail{RepoName: "repo1"}
				token := base58.Encode(util.ToBytes(txDetail))
				req := httptest.NewRequest("POST", "https://127.0.0.1", bytes.NewReader(nil))
				req.SetBasicAuth(token, "")
				svr.authenticate = func([]*types.TxDetail, *state.Repository, *state.Namespace, core.Keepers, validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, fmt.Errorf("auth error")
				}
				_, _, err := svr.handleAuth(req, w, &state.Repository{}, &state.Namespace{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("auth error"))
			})
		})

		When("a push token is ok and authentication passes", func() {
			It("should return enforcer", func() {
				txDetail := &types.TxDetail{RepoName: "repo1"}
				token := base58.Encode(util.ToBytes(txDetail))
				req := httptest.NewRequest("POST", "https://127.0.0.1", bytes.NewReader(nil))
				req.SetBasicAuth(token, "")
				enforcer := policy.GetPolicyEnforcer([][]*state.Policy{{{Object: "obj", Subject: "sub", Action: "ac"}}})
				svr.authenticate = func([]*types.TxDetail, *state.Repository, *state.Namespace, core.Keepers, validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return enforcer, nil
				}
				_, enc, err := svr.handleAuth(req, w, &state.Repository{}, &state.Namespace{})
				Expect(err).To(BeNil())
				Expect(fmt.Sprintf("%p", enc)).To(Equal(fmt.Sprintf("%p", enforcer)))
			})
		})
	})

	Describe(".CheckPolicy", func() {
		It("should return error when reference type is unknown", func() {
			enforcer := policy.GetPolicyEnforcer([][]*state.Policy{{{Object: "obj", Subject: "sub", Action: "ac"}}})
			err := policy.CheckPolicy(enforcer, "refs/unknown/xyz", false, key.PushAddr().String(), false, "write")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unknown reference (refs/unknown/xyz)"))
		})

		Context("with 'write' action", func() {
			var allowAction = "write"
			var denyAction = "deny-" + allowAction
			var enforcer policy.EnforcerFunc
			var pushAddrA string

			BeforeEach(func() {
				pushAddrA = key.PushAddr().String()
			})

			When("action is allowed on any level", func() {
				It("should return nil at level 0", func() {
					policies := [][]*state.Policy{{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
				It("should return nil at level 1", func() {
					policies := [][]*state.Policy{{}, {{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
				It("should return nil at level 2", func() {
					policies := [][]*state.Policy{{}, {}, {{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}}}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
			})

			When("action does not have a policy", func() {
				It("should return err", func() {
					policies := [][]*state.Policy{{}, {}, {}}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
			})

			When("action is denied on level 0 and allowed on level 1", func() {
				It("should return err", func() {
					policies := [][]*state.Policy{
						{{Subject: pushAddrA, Object: "refs/heads/master", Action: denyAction}},
						{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
					}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err.Error()).To(Equal("reference (refs/heads/master): not authorized to perform 'write' action"))
				})
			})

			When("action is allowed for subject:'all' on level 2", func() {
				It("should return nil", func() {
					policies := [][]*state.Policy{
						{}, {},
						{{Subject: "all", Object: "refs/heads/master", Action: allowAction}},
					}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
			})

			When("action is denied for subject:'all' on level 2", func() {
				It("should return error", func() {
					policies := [][]*state.Policy{
						{}, {},
						{{Subject: "all", Object: "refs/heads/master", Action: denyAction}},
					}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
			})

			When("action is denied on dir:refs/heads as subject:'all' on level 0 and allowed on refs/heads/master on level 1", func() {
				It("should not authorize pushAddrA by returning error", func() {
					policies := [][]*state.Policy{
						{{Subject: pushAddrA, Object: "refs/heads", Action: denyAction}},
						{{Subject: pushAddrA, Object: "refs/heads/master", Action: allowAction}},
					}
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/heads/master", false, pushAddrA, false, allowAction)
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
					enforcer = policy.GetPolicyEnforcer(policies)
					err = policy.CheckPolicy(enforcer, "refs/tags/tag1", false, pushAddrA, false, allowAction)
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".DecodePushToken", func() {
		When("token is malformed (not base58 encoded)", func() {
			It("should return err", func() {
				_, err := DecodePushToken("invalid_token")
				Expect(err).To(MatchError("malformed token"))
			})
		})

		When("token is malformed (can't be deserialized to TxDetail)", func() {
			It("should return err", func() {
				_, err := DecodePushToken(base58.Encode([]byte("invalid data")))
				Expect(err).To(MatchError("malformed token"))
			})
		})

		When("token is valid", func() {
			It("should return no error and transaction detail object", func() {
				txDetail := &types.TxDetail{RepoName: "repo1"}
				token := base58.Encode(txDetail.Bytes())
				res, err := DecodePushToken(token)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(txDetail))
			})
		})
	})

	Describe(".MakePushToken", func() {
		var token string
		var txDetail *types.TxDetail

		BeforeEach(func() {
			txDetail = &types.TxDetail{RepoName: "repo1"}
			mockStoreKey := mocks.NewMockStoredKey(ctrl)
			mockStoreKey.EXPECT().GetKey().Return(key)
			token = MakePushToken(mockStoreKey, txDetail)
		})

		It("should return token", func() {
			Expect(token).ToNot(BeEmpty())
		})

		It("should decode token successfully", func() {
			txD, err := DecodePushToken(token)
			Expect(err).To(BeNil())
			Expect(txD).To(Equal(txDetail))
		})
	})

	Describe(".UpdateRemoteURLsWithPushToken", func() {
		var mockRepo *mocks.MockLocalRepo
		var txDetail *types.TxDetail
		var mockStoreKey *mocks.MockStoredKey
		var token string

		BeforeEach(func() {
			mockRepo = mocks.NewMockLocalRepo(ctrl)
			txDetail = &types.TxDetail{RepoName: "repo1"}
			mockStoreKey = mocks.NewMockStoredKey(ctrl)
		})

		It("should return err when unable to get config", func() {
			mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error"))
			_, err = UpdateRemoteURLsWithPushToken(mockRepo, "master", txDetail, mockStoreKey, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get config: error"))
		})

		It("should return err when unable to get config", func() {
			cfg := &gogitcfg.Config{}
			mockRepo.EXPECT().Config().Return(cfg, nil)
			_, err = UpdateRemoteURLsWithPushToken(mockRepo, "master", txDetail, mockStoreKey, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("remote (master): does not exist"))
		})

		It("should return err when remote urls point to different namespaces", func() {
			mockStoreKey.EXPECT().GetKey().Return(key)
			cfg := &gogitcfg.Config{Remotes: map[string]*gogitcfg.RemoteConfig{}}
			cfg.Remotes["master"] = &gogitcfg.RemoteConfig{
				URLs: []string{"https://push.node/r/repo1", "https://push.node/ns/repo1"},
			}
			mockRepo.EXPECT().Config().Return(cfg, nil)
			_, err = UpdateRemoteURLsWithPushToken(mockRepo, "master", txDetail, mockStoreKey, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("remote (master): cannot have multiple urls pointing to different repository/namespace"))
		})

		It("should return err when remote urls point to different repos", func() {
			mockStoreKey.EXPECT().GetKey().Return(key)
			cfg := &gogitcfg.Config{Remotes: map[string]*gogitcfg.RemoteConfig{}}
			cfg.Remotes["master"] = &gogitcfg.RemoteConfig{
				URLs: []string{"https://push.node/ns/repo1", "https://push.node/ns/repo2"},
			}
			mockRepo.EXPECT().Config().Return(cfg, nil)
			_, err = UpdateRemoteURLsWithPushToken(mockRepo, "master", txDetail, mockStoreKey, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("remote (master): cannot have multiple urls pointing to different repository/namespace"))
		})

		When("on success", func() {
			BeforeEach(func() {
				mockStoreKey.EXPECT().GetKey().Return(key).Times(2)
				cfg := &gogitcfg.Config{Remotes: map[string]*gogitcfg.RemoteConfig{}}
				cfg.Remotes["master"] = &gogitcfg.RemoteConfig{
					URLs: []string{"https://push.node/r/repo1", "https://push.node/r/repo1"},
				}
				mockRepo.EXPECT().Config().Return(cfg, nil)
				mockRepo.EXPECT().SetConfig(gomock.Any()).Return(nil)
				token, err = UpdateRemoteURLsWithPushToken(mockRepo, "master", txDetail, mockStoreKey, false)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
				Expect(token).ToNot(BeEmpty())
			})

			It("should set token to *_LAST_PUSH_TOKEN env var", func() {
				envToken := os.Getenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName))
				Expect(envToken).To(Equal(token))
			})
		})
	})
})
