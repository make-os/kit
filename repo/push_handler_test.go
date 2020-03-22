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
	"gitlab.com/makeos/mosdef/types/core"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
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
		// TODO: Use real &TxParams{} and polEnforcer
		handler = newPushHandler(repo, &PushRequestTokenData{}, nil, mockMgr)
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
				Expect(err.Error()).To(Equal("failed to read pushed update: invalid pkt-len found"))
			})
		})
	})

	Describe(".HandleReferences", func() {
		When("old state is not set", func() {
			BeforeEach(func() {
				_, _, err = handler.HandleReferences()
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("expected old state to have been captured"))
			})
		})

		When("txparams was not set", func() {
			var err error
			BeforeEach(func() {
				handler.rMgr = mgr
				oldState := getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				newState := getRepoState(repo)
				var packfile io.ReadSeeker
				packfile, err = makePackfile(repo, oldState, newState)

				Expect(err).To(BeNil())
				handler.oldState = oldState
				err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
				Expect(err).To(BeNil())

				_, _, err = handler.HandleReferences()
			})

			It("should return err='validation error.*txparams was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("transaction params was not set"))
			})
		})

		When("references are handled without error", func() {
			var err error
			BeforeEach(func() {
				handler.referenceHandler = func(ref string) (params *util.TxParams, errors []error) {
					return &util.TxParams{}, nil
				}
				handler.oldState = getRepoState(repo)
				handler.pushReader = &PushReader{
					references: packedReferences{{name: "refs/branch/master"}, {name: "refs/branch/dev"}},
				}
				_, _, err = handler.HandleReferences()
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("a reference returns an error", func() {
			var err error
			BeforeEach(func() {
				called := 0
				handler.referenceHandler = func(ref string) (params *util.TxParams, errors []error) {
					if called == 0 {
						called++
						errors = []error{fmt.Errorf("bad reference")}
						return
					}
					return &util.TxParams{}, nil
				}
				handler.oldState = getRepoState(repo)
				handler.pushReader = &PushReader{
					references: packedReferences{{name: "refs/branch/master"}, {name: "refs/branch/dev"}},
				}
				_, _, err = handler.HandleReferences()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad reference"))
			})
		})

		When("references have different push key ID", func() {
			var err error
			BeforeEach(func() {
				called := 0
				handler.referenceHandler = func(ref string) (params *util.TxParams, errors []error) {
					called++
					if called == 1 {
						return &util.TxParams{PushKeyID: "push1_abc"}, nil
					}
					return &util.TxParams{PushKeyID: "push1_xyz"}, nil
				}
				handler.oldState = getRepoState(repo)
				handler.pushReader = &PushReader{
					references: packedReferences{{name: "refs/branch/master"}, {name: "refs/branch/dev"}},
				}
				_, _, err = handler.HandleReferences()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("rejected because the pushed references were signed with different push keys"))
			})
		})
	})

	Describe(".handleReference", func() {
		var errs []error
		Describe("when unable to get state of the repository", func() {
			BeforeEach(func() {
				handler.oldState = getRepoState(repo)
				mockMgr.EXPECT().GetRepoState(repo, matchOpt("refs/heads/master")).Return(nil, fmt.Errorf("error"))
				_, errs = handler.handleReference("refs/heads/master")
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("failed to get current state: error"))
			})
		})

		Describe("when a reference failed change validation", func() {
			BeforeEach(func() {
				handler.rMgr = mgr
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 2")
				handler.changeValidator = func(repo core.BareRepo, change *core.ItemChange, pushKeyGetter core.PushKeyGetter) (params *util.TxParams, err error) {
					return nil, fmt.Errorf("bad reference change")
				}
				_, errs = handler.handleReference("refs/heads/master")
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

		Describe("when a reference failed change validation and reversion failed", func() {
			BeforeEach(func() {
				handler.rMgr = mgr
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 2")
				handler.changeValidator = func(repo core.BareRepo, change *core.ItemChange, pushKeyGetter core.PushKeyGetter) (params *util.TxParams, err error) {
					return nil, fmt.Errorf("bad reference change")
				}
				handler.reverter = func(repo core.BareRepo, prevState core.BareRepoState, options ...core.KVOption) (changes *core.Changes, err error) {
					return nil, fmt.Errorf("failed revert")
				}
				_, errs = handler.handleReference("refs/heads/master")
			})

			It("should return 2 error", func() {
				Expect(errs).To(HaveLen(2))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
				Expect(errs[1].Error()).To(Equal("failed to revert to old state: failed revert"))
			})
		})

		Describe("when a reference failed change validation and reversion failed", func() {
			BeforeEach(func() {
				handler.rMgr = mgr
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 2")
				handler.changeValidator = func(repo core.BareRepo, change *core.ItemChange, pushKeyGetter core.PushKeyGetter) (params *util.TxParams, err error) {
					return nil, fmt.Errorf("bad reference change")
				}
				handler.reverter = func(repo core.BareRepo, prevState core.BareRepoState, options ...core.KVOption) (changes *core.Changes, err error) {
					return nil, fmt.Errorf("failed revert")
				}
				_, errs = handler.handleReference("refs/heads/master")
			})

			It("should return 2 error", func() {
				Expect(errs).To(HaveLen(2))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
				Expect(errs[1].Error()).To(Equal("failed to revert to old state: failed revert"))
			})
		})

		Describe("when merge id is set in transaction info but merge compliance failed", func() {
			BeforeEach(func() {
				handler.rMgr = mgr
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.oldState = getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 2")
				handler.changeValidator = func(repo core.BareRepo, change *core.ItemChange, pushKeyGetter core.PushKeyGetter) (params *util.TxParams, err error) {
					return &util.TxParams{
						MergeProposalID: "1000111",
					}, nil
				}
				handler.mergeChecker = func(repo core.BareRepo, change *core.ItemChange, oldRef core.Item, mergeProposalID, pushKeyID string, keepers core.Keepers) error {
					return fmt.Errorf("failed merge check")
				}
				_, errs = handler.handleReference("refs/heads/master")
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): failed merge check"))
			})
		})
	})
})
