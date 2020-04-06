package repo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

var _ = Describe("PushHandler", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo core.BareRepo
	var mockMgr *mocks.MockRepoManager
	var mgr *Manager
	var handler *PushHandler
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var repoName string
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockLogic = mocks.NewMockLogic(ctrl)
		mockDHT := mocks.NewMockDHTNode(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mgr = NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)
		mockMgr = mocks.NewMockRepoManager(ctrl)
		mockMgr.EXPECT().Log().Return(cfg.G().Log)

		handler = newPushHandler(repo, []*types.TxDetail{}, nil, mockMgr)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".HandleStream", func() {
		When("unable to get repo old state", func() {
			BeforeEach(func() {
				mockMgr.EXPECT().GetRepoState(repo).Return(nil, fmt.Errorf("error"))
				err = handler.HandleStream(nil, nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})

		When("packfile is invalid", func() {
			BeforeEach(func() {
				oldState := &State{}
				mockMgr.EXPECT().GetRepoState(repo).Return(oldState, nil)
				err = handler.HandleStream(strings.NewReader("invalid"), nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to decode request pack: invalid pkt-len found"))
			})
		})

		When("packfile is valid", func() {
			var packfile io.ReadSeeker
			BeforeEach(func() {
				oldState := getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				newState := getRepoState(repo)
				packfile, err = makePackfile(repo, oldState, newState)
				Expect(err).To(BeNil())
				handler.oldState = oldState
			})

			When("authorization failed", func() {
				It("should return error returned from the authorization handler", func() {
					handler.authorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return fmt.Errorf("auth failed badly")
					}
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("authorization: auth failed badly"))
				})
			})

			When("authorization succeeds", func() {
				It("should return no error", func() {
					handler.authorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return nil
					}
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".checkForReferencesTxDetail", func() {
		It("should return error if a reference has no tx detail", func() {
			handler.pushReader.references = map[string]*packedReferenceObject{"refs/heads/master": {}}
			err := handler.checkForReferencesTxDetail()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("reference (refs/heads/master) has no transaction information"))
		})

		It("should return no error if a reference has tx detail", func() {
			handler.pushReader.references = map[string]*packedReferenceObject{"refs/heads/master": {}}
			handler.txDetails["refs/heads/master"] = &types.TxDetail{}
			err := handler.checkForReferencesTxDetail()
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleAuthorization", func() {
		var ur *packp.ReferenceUpdateRequest
		BeforeEach(func() {
			handler.pushReader.references = map[string]*packedReferenceObject{"refs/heads/master": {}}
			ur = &packp.ReferenceUpdateRequest{}
		})

		It("should return error if a reference has no tx detail", func() {
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("reference (refs/heads/master) has no transaction information"))
		})

		It("should return error when command is a delete request and policy check failed", func() {
			handler.txDetails["refs/heads/master"] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: plumbing.ZeroHash})
			handler.policyChecker = func(enforcer policyEnforcer, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("delete"))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return error when command is an update request and policy check failed", func() {
			handler.txDetails["refs/heads/master"] = &types.TxDetail{}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: hash})
			handler.policyChecker = func(enforcer policyEnforcer, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("update"))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return nil when policy check passes", func() {
			handler.txDetails["refs/heads/master"] = &types.TxDetail{}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: hash})
			handler.policyChecker = func(enforcer policyEnforcer, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("update"))
				return nil
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).To(BeNil())
		})
	})

	Describe(".announceObject", func() {
		var mockDHT *mocks.MockDHTNode
		BeforeEach(func() {
			handler.mgr = mockMgr
			mockDHT = mocks.NewMockDHTNode(ctrl)
			mockMgr.EXPECT().GetDHT().Return(mockDHT)
		})

		It("should return error when DHT failed to announce", func() {
			mockDHT.EXPECT().Announce(gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to announce"))
			err = handler.announceObject(util.RandString(40))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to announce"))
		})

		It("should return no error when DHT did not fail", func() {
			mockDHT.EXPECT().Announce(gomock.Any(), gomock.Any()).Return(nil)
			err = handler.announceObject(util.RandString(40))
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleReferences", func() {
		When("old state is not set", func() {
			BeforeEach(func() {
				err = handler.HandleReferences()
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("expected old state to have been captured"))
			})
		})

		When("reference handler function returns an error", func() {
			var err error
			BeforeEach(func() {
				handler.oldState = getRepoState(repo)
				handler.referenceHandler = func(ref string) []error {
					return []error{fmt.Errorf("bad error")}
				}
				handler.pushReader.references = map[string]*packedReferenceObject{
					"refs/heads/master": {},
				}

				err = handler.HandleReferences()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})
	})

	Describe(".handleReference", func() {
		var errs []error
		Describe("when unable to get state of the repository", func() {
			BeforeEach(func() {
				handler.oldState = getRepoState(repo)
				mockMgr.EXPECT().GetRepoState(repo, matchOpt("refs/heads/master")).Return(nil, fmt.Errorf("error"))
				errs = handler.handleReference("refs/heads/master")
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("failed to get current state: error"))
			})
		})

		Describe("when a reference failed change validation check", func() {
			BeforeEach(func() {
				handler.mgr = mgr
				handler.pushReader.references = map[string]*packedReferenceObject{
					"refs/heads/master": {newHash: util.RandString(40)},
				}
				handler.changeValidator = func(core.BareRepo, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
					return fmt.Errorf("bad reference change")
				}

				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				errs = handler.handleReference("refs/heads/master")
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
			})

			It("should revert the reference back to the old state", func() {
				newState := getRepoState(repo)
				Expect(newState).To(Equal(handler.oldState))
			})
		})

		Context("check reversion", func() {
			Describe("when a reference failed change validation and reversion failed", func() {
				BeforeEach(func() {
					handler.mgr = mgr
					handler.pushReader.references = map[string]*packedReferenceObject{
						"refs/heads/master": {newHash: util.RandString(40)},
					}
					handler.changeValidator = func(core.BareRepo, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return fmt.Errorf("bad reference change")
					}
					handler.reverter = func(repo core.BareRepo, prevState core.BareRepoState, options ...core.KVOption) (changes *core.Changes, err error) {
						return nil, fmt.Errorf("failed revert")
					}

					appendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.oldState = getRepoState(repo)
					appendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.handleReference("refs/heads/master")
				})

				It("should return 2 error", func() {
					Expect(errs).To(HaveLen(2))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
					Expect(errs[1].Error()).To(Equal("failed to revert to old state: failed revert"))
				})
			})

			Describe("when a reference failed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.mgr = mgr
					handler.pushReader.references = map[string]*packedReferenceObject{
						"refs/heads/master": {newHash: util.RandString(40)},
					}
					handler.changeValidator = func(core.BareRepo, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return fmt.Errorf("bad reference change")
					}

					appendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.oldState = getRepoState(repo)
					appendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.handleReference("refs/heads/master")
				})

				It("should return 1 error", func() {
					Expect(errs).To(HaveLen(1))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := getRepoState(repo)
					Expect(curState).To(Equal(handler.oldState))
				})
			})

			Describe("when a reference passed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.mgr = mgr
					handler.txDetails = map[string]*types.TxDetail{
						"refs/heads/master": {},
					}
					handler.pushReader.references = map[string]*packedReferenceObject{
						"refs/heads/master": {newHash: util.RandString(40)},
					}
					handler.changeValidator = func(core.BareRepo, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return nil
					}

					appendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.oldState = getRepoState(repo)
					appendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.handleReference("refs/heads/master")
				})

				It("should return no error", func() {
					Expect(errs).To(HaveLen(0))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := getRepoState(repo)
					Expect(curState).To(Equal(handler.oldState))
				})
			})
		})

		Describe("when merge id is set in transaction info but merge compliance failed", func() {
			BeforeEach(func() {
				handler.mgr = mgr
				handler.txDetails = map[string]*types.TxDetail{
					"refs/heads/master": {MergeProposalID: "001"},
				}
				handler.pushReader.references = map[string]*packedReferenceObject{
					"refs/heads/master": {newHash: strings.Repeat("0", 40)},
				}
				handler.changeValidator = func(core.BareRepo, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
					return nil
				}
				handler.mergeChecker = func(repo core.BareRepo, change *core.ItemChange, oldRef core.Item, mergeProposalID, pushKeyID string, keepers core.Keepers) error {
					return fmt.Errorf("failed merge check")
				}

				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 2")
				errs = handler.handleReference("refs/heads/master")
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): failed merge check"))
			})
		})
	})
})
