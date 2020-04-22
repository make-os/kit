package pushhandler_test

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
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/pushhandler"
	repo3 "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/server"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
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
	var svr core.RemoteServer
	var handler *pushhandler.PushHandler
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
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetRepoWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockLogic = mocks.NewMockLogic(ctrl)
		mockDHT := mocks.NewMockDHTNode(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		svr = server.NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)
		mockMgr = mocks.NewMockRepoManager(ctrl)
		mockMgr.EXPECT().Log().Return(cfg.G().Log)

		handler = pushhandler.NewHandler(repo, []*types.TxDetail{}, nil, mockMgr)
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
				oldState := &plumbing2.State{}
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
				oldState := plumbing2.GetRepoState(repo)
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				newState := plumbing2.GetRepoState(repo)
				packfile, err = pushhandler.MakePackfile(repo, oldState, newState)
				Expect(err).To(BeNil())
				handler.OldState = oldState
			})

			When("authorization failed", func() {
				It("should return error returned from the authorization handler", func() {
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return fmt.Errorf("auth failed badly")
					}
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("authorization: auth failed badly"))
				})
			})

			When("authorization succeeds", func() {
				It("should return no error", func() {
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return nil
					}
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".CheckForReferencesTxDetail", func() {
		It("should return error if a reference has no tx detail", func() {
			handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{"refs/heads/master": {}}
			err := handler.CheckForReferencesTxDetail()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("reference (refs/heads/master) has no transaction information"))
		})

		It("should return no error if a reference has tx detail", func() {
			handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{"refs/heads/master": {}}
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{}
			err := handler.CheckForReferencesTxDetail()
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleAuthorization", func() {
		var ur *packp.ReferenceUpdateRequest
		BeforeEach(func() {
			handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{"refs/heads/master": {}}
			ur = &packp.ReferenceUpdateRequest{}
		})

		It("should return no error if a tx detail include a merge ID", func() {
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{MergeProposalID: "xyz"}
			err = handler.HandleAuthorization(ur)
			Expect(err).To(BeNil())
		})

		It("should return error when command is a delete request and policy check failed", func() {
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: plumbing.ZeroHash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("delete"))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return error when command is a merge update request and policy check failed", func() {
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{MergeProposalID: "123"}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("merge-update"))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return error when command is a issue update request and policy check failed", func() {
			handler.PushReader.References["refs/heads/issues/do-something"] = &pushhandler.PackedReferenceObject{}
			handler.TxDetails["refs/heads/issues/do-something"] = &types.TxDetail{MergeProposalID: "123"}
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{MergeProposalID: "234"}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/issues/do-something", New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/issues/do-something"))
				Expect(action).To(Or(Equal("issue-update"), Equal("merge-update")))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return error when command is an update request and policy check failed", func() {
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("update"))
				return fmt.Errorf("unauthorized")
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unauthorized"))
		})

		It("should return nil when policy check passes", func() {
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: "refs/heads/master", New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, pushKeyID, reference, action string) error {
				Expect(reference).To(Equal("refs/heads/master"))
				Expect(action).To(Equal("update"))
				return nil
			}
			err = handler.HandleAuthorization(ur)
			Expect(err).To(BeNil())
		})
	})

	Describe(".AnnounceObject", func() {
		var mockDHT *mocks.MockDHTNode
		BeforeEach(func() {
			handler.Server = mockMgr
			mockDHT = mocks.NewMockDHTNode(ctrl)
			mockMgr.EXPECT().GetDHT().Return(mockDHT)
		})

		It("should return error when DHT failed to announce", func() {
			mockDHT.EXPECT().Announce(gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to announce"))
			err = handler.AnnounceObject(util.RandString(40))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to announce"))
		})

		It("should return no error when DHT did not fail", func() {
			mockDHT.EXPECT().Announce(gomock.Any(), gomock.Any()).Return(nil)
			err = handler.AnnounceObject(util.RandString(40))
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
				handler.OldState = plumbing2.GetRepoState(repo)
				handler.ReferenceHandler = func(ref string, revertOnly bool) []error {
					return []error{fmt.Errorf("bad error")}
				}
				handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
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

	Describe(".HandleReference", func() {
		var errs []error
		Describe("when unable to get state of the repository", func() {
			BeforeEach(func() {
				handler.OldState = plumbing2.GetRepoState(repo)
				mockMgr.EXPECT().GetRepoState(repo, plumbing2.MatchOpt("refs/heads/master")).Return(nil, fmt.Errorf("error"))
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("failed to get current state: error"))
			})
		})

		Describe("when a reference failed change validation check", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
					"refs/heads/master": {NewHash: util.RandString(40)},
				}
				handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
					return fmt.Errorf("bad reference change")
				}

				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
			})

			It("should revert the reference back to the old state", func() {
				newState := plumbing2.GetRepoState(repo)
				Expect(newState).To(Equal(handler.OldState))
			})
		})

		Context("check reversion", func() {
			Describe("when a reference failed change validation and reversion failed", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
						"refs/heads/master": {NewHash: util.RandString(40)},
					}
					handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return fmt.Errorf("bad reference change")
					}
					handler.Reverter = func(repo core.BareRepo, prevState core.BareRepoState, options ...core.KVOption) (changes *core.Changes, err error) {
						return nil, fmt.Errorf("failed revert")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return 2 error", func() {
					Expect(errs).To(HaveLen(2))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
					Expect(errs[1].Error()).To(Equal("failed to revert to old state: failed revert"))
				})
			})

			Describe("when a reference failed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
						"refs/heads/master": {NewHash: util.RandString(40)},
					}
					handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return fmt.Errorf("bad reference change")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return 1 error", func() {
					Expect(errs).To(HaveLen(1))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
				})
			})

			Describe("when a reference failed change validation but revertOnly is true", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
						"refs/heads/master": {NewHash: util.RandString(40)},
					}
					handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return fmt.Errorf("bad reference change")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", true)
				})

				It("should return no error since validation is skipped", func() {
					Expect(errs).To(HaveLen(0))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
				})
			})

			Describe("when a reference passed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.TxDetails = map[string]*types.TxDetail{
						"refs/heads/master": {},
					}
					handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
						"refs/heads/master": {NewHash: util.RandString(40)},
					}
					handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
						return nil
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return no error", func() {
					Expect(errs).To(HaveLen(0))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
				})
			})
		})

		Describe("when merge id is set in transaction info but merge compliance failed", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.TxDetails = map[string]*types.TxDetail{
					"refs/heads/master": {MergeProposalID: "001"},
				}
				handler.PushReader.References = map[string]*pushhandler.PackedReferenceObject{
					"refs/heads/master": {NewHash: strings.Repeat("0", 40)},
				}
				handler.ChangeValidator = func(core.BareRepo, string, *core.ItemChange, *types.TxDetail, core.PushKeyGetter) (err error) {
					return nil
				}
				handler.MergeChecker = func(repo core.BareRepo, change *core.ItemChange, oldRef core.Item, mergeProposalID, pushKeyID string, keepers core.Keepers) error {
					return fmt.Errorf("failed merge check")
				}

				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): failed merge check"))
			})
		})
	})
})
