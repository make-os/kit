package validation_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/logic/contracts/mergerequest"
	"gitlab.com/makeos/mosdef/mocks"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/types/common"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var commit *mocks.MockCommit

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		commit = mocks.NewMockCommit(ctrl)
		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ValidatePostCommit", func() {
		var detail *types.TxDetail
		var args *validation.ValidatePostCommitArg

		BeforeEach(func() {
			detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
			args = &validation.ValidatePostCommitArg{OldHash: "", TxDetail: detail}
		})

		It("should return error when unable to check for merge commits", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			detail := &types.TxDetail{Reference: "refs/heads/issues/1"}
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change, TxDetail: detail}
			mockRepo.EXPECT().HasMergeCommits(detail.Reference).Return(false, fmt.Errorf("error"))
			err := validation.ValidatePostCommit(mockRepo, nil, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check for merge commits in post reference: error"))
		})

		It("should return error when issue has merge commits", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
			args = &validation.ValidatePostCommitArg{OldHash: "", Change: change, TxDetail: detail}
			commit := repo.WrapCommit(&object.Commit{})
			mockRepo.EXPECT().HasMergeCommits(detail.Reference).Return(true, nil)
			err := validation.ValidatePostCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("post history must not include merge commits"))
		})

		It("should return error when commit failed commit validation", func() {
			commit := repo.WrapCommit(&object.Commit{})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change, TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("check error")
				},
			}
			err := validation.ValidatePostCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("check error"))
		})

		It("should return error when unable to get ancestors", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			detail := &types.TxDetail{
				Reference: "refs/heads/issues/1",
			}

			mockRepo.EXPECT().GetAncestors(commit.UnWrap(), args.OldHash, true).Return(nil, fmt.Errorf("ancestor get error"))
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
				TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				},
				CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
					return nil, nil
				},
			}
			err := validation.ValidatePostCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("ancestor get error"))
		})

		It("should return error when commit failed issue commit validation ", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			detail := &types.TxDetail{Reference: "refs/heads/issues/1"}
			mockRepo.EXPECT().GetAncestors(commit.UnWrap(), args.OldHash, true).Return([]*object.Commit{}, nil)
			mockRepoState := state.BareRepository()
			mockRepo.EXPECT().GetState().Return(mockRepoState)

			callCount := 0
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
				TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				},
				CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
					callCount++
					Expect(commit).To(Equal(commit))
					Expect(args.Reference).To(Equal(detail.Reference))
					return nil, fmt.Errorf("issue check error")
				},
			}
			err := validation.ValidatePostCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue check error"))
			Expect(callCount).To(Equal(1))
		})

		When("commit has no ancestor", func() {
			var commitObj *object.Commit
			var mockRepoState *state.Repository

			BeforeEach(func() {
				commitObj = &object.Commit{}
				commit.EXPECT().UnWrap().Return(commitObj)
				mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
				mockRepo.EXPECT().GetAncestors(commitObj, args.OldHash, true).Return([]*object.Commit{}, nil)
				mockRepoState = state.BareRepository()
				mockRepo.EXPECT().GetState().Return(mockRepoState)
			})

			It("should return no error when issue commit check is passed and"+
				"issue checker func must be called once", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: &types.TxDetail{Reference: "refs/heads/issues/1"},
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.PostBody{}, nil
					},
				}
				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
			})

			It("should set tx detail FlagCheckIssueUpdatePolicy to true if post body updates admin fields like 'labels'", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.PostBody{
							IssueFields: common.IssueFields{
								Labels: &[]string{"label_update"},
							},
						}, nil
					},
				}
				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
				Expect(detail.FlagCheckAdminUpdatePolicy).To(BeTrue())
			})

			It("should populate tx detail reference data fields from post body", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						cls := true
						return &plumbing2.PostBody{
							Close: &cls,
							IssueFields: common.IssueFields{
								Labels:    &[]string{"l1", "l2"},
								Assignees: &[]string{"key1", "key2"},
							},
						}, nil
					},
				}

				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
				Expect(*detail.Data().Close).To(Equal(true))
				Expect(*detail.Data().Labels).To(Equal([]string{"l1", "l2"}))
				Expect(*detail.Data().Assignees).To(Equal([]string{"key1", "key2"}))
			})

			It("should return error when issue reference has been previously closed and new issue commit did not set close=2", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{
					Hash: []byte("hash"),
					Data: &state.ReferenceData{Closed: true},
				}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.PostBody{}, nil
					},
				}

				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrCannotWriteToClosedRef))
			})

			It("should return no error when issue reference has been previously closed and new issue commit set close=2", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{Data: &state.ReferenceData{Closed: true}, Hash: []byte("hash")}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						cls := false
						return &plumbing2.PostBody{Close: &cls}, nil
					},
				}

				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
			})
		})

		When("commit has an ancestor", func() {
			var child, ancestor *object.Commit

			BeforeEach(func() {
				child = &object.Commit{Hash: plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")}
				commit.EXPECT().UnWrap().Return(child)
				mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
				ancestor = &object.Commit{Hash: plumbing.NewHash("c045fafe22ae2ef4d7c2390704e9b7a73c12bd43")}
				mockRepo.EXPECT().GetAncestors(child, args.OldHash, true).Return([]*object.Commit{ancestor}, nil)
				mockRepoState := state.BareRepository()
				mockRepo.EXPECT().GetState().Return(mockRepoState).Times(1)
			})

			Specify("that issue checker is called twice for both the commit and its ancestor", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: child.Hash.String()}}
				callCount := 0
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: &types.TxDetail{Reference: "refs/heads/issues/1"},
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						callCount++

						if callCount == 1 {
							Expect(commit.UnWrap()).To(Equal(ancestor))
							Expect(args.IsNew).To(BeTrue())
						} else {
							Expect(commit.UnWrap()).To(Equal(child))
							Expect(args.IsNew).To(BeFalse())
						}

						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.PostBody{}, nil
					},
				}
				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(2))
			})
		})
	})

	Describe(".CheckPostCommit", func() {
		It("should return error when issue number is not valid", func() {
			args := &validation.CheckPostCommitArgs{Reference: "refs/heads/" + plumbing2.IssueBranchPrefix + "/abc"}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post number is not valid. Must be numeric"))
		})

		It("should return error when issue commit has more than 1 parents", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(2)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post commit cannot have more than one parent"))
		})

		It("should return error when the reference of the issue commit is new but the issue commit has multiple parents ", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1).Times(2)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch, IsNew: true}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("first commit of a new post must have no parent"))
		})

		It("should return error when the issue commit alters history", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post commit must not alter history"))
		})

		It("should return error when unable to get commit tree", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			commit.EXPECT().GetTree().Return(nil, fmt.Errorf("bad query"))
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("unable to read post commit tree"))
		})

		It("should return error when issue commit tree does not have 'body' file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post commit must have a 'body' file"))
		})

		It("should return error when issue commit tree has more than 1 files", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{}, {}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post commit tree must only include a 'body' file"))
		})

		It("should return error when issue commit tree has a body entry that isn't a regular file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{Name: "body", Mode: filemode.Dir}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckPostCommitArgs{Reference: issueBranch}
			_, err := validation.CheckPostCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("post body file is not a regular file"))
		})
	})

	Describe(".CheckPostBody", func() {
		var commit *object.Commit
		var wc *repo.WrappedCommit

		BeforeEach(func() {
			commit = &object.Commit{Hash: plumbing2.MakeCommitHash("hash")}
			wc = repo.WrapCommit(commit)
		})

		It("should return error when an unexpected issue field exists", func() {
			ref := plumbing2.MakeIssueReference(1)
			err := validation.CheckPostBody(nil, ref, wc, true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.field1, msg:unexpected field"))
		})

		It("should return error when an issue reference type is unknown", func() {
			ref := "refs/heads/unknown"
			err := validation.CheckPostBody(nil, ref, repo.WrapCommit(commit), true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("unsupported post type"))
		})

		Context("common post body check", func() {
			var ref = plumbing2.MakeIssueReference(1)

			It("should return error when 'title' is not string", func() {
				fm := map[string]interface{}{"title": 123}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:expected a string value"))
			})

			It("should return error when 'replyTo' is not string", func() {
				fm := map[string]interface{}{"replyTo": 123}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:expected a string value"))
			})

			It("should return error when 'reactions' is not string", func() {
				fm := map[string]interface{}{"reactions": "smile"}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions," +
					" msg:expected a list of string values"))
			})

			It("should return error when 'replyTo' is set in a new reference", func() {
				fm := map[string]interface{}{"replyTo": "0x123"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, " +
					"msg:not expected in a new post commit"))
			})

			It("should return error when 'title' is set in a reply", func() {
				fm := map[string]interface{}{"replyTo": "0x123", "title": "a title"}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, " +
					"msg:title is not required when replying"))
			})

			It("should return error when 'title' is not provided in new reference", func() {
				fm := map[string]interface{}{}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is required"))
			})

			It("should return error when 'title' is provided but reference is not new", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is not required when replying"))
			})

			It("should return error when 'title' exceeds max. characters", func() {
				fm := map[string]interface{}{"title": strings.Repeat("a", validation.MaxIssueTitleLen+1)}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is too long; cannot exceed .* characters"))
			})

			It("should return error when 'replyTo' length is too low or too high", func() {
				fm := map[string]interface{}{"replyTo": "0x1"}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))
				fm = map[string]interface{}{"replyTo": strings.Repeat("a", 41)}
				err = validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))
			})

			It("should return error when 'replyTo' hash does not point to an ancestor", func() {
				ancestor := "hash_of_ancestor"
				mockRepo.EXPECT().IsAncestor(ancestor, commit.Hash.String()).Return(fmt.Errorf("error"))
				fm := map[string]interface{}{"replyTo": "hash_of_ancestor"}
				err := validation.CheckPostBody(mockRepo, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:hash is not a known ancestor"))
			})

			It("should return error when 'reaction' values exceed max", func() {
				fm := map[string]interface{}{"reactions": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:too many reactions; cannot exceed 10"))
			})

			It("should return error when 'reaction' does not contain string entries", func() {
				fm := map[string]interface{}{"reactions": []interface{}{1}}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:expected a string list"))
			})

			It("should return error when 'reaction' includes an unknown reaction", func() {
				fm := map[string]interface{}{"reactions": []interface{}{"unknown_reaction"}}
				err := validation.CheckPostBody(nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.reactions, msg:reaction 'unknown_reaction' is unknown"))
			})

			It("should return error when reference is new and 'content' is unset", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:post content is required"))
			})

			It("should return error when reference is new and 'content' exceeds max", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, bytes.Repeat([]byte{1}, validation.MaxIssueContentLen+1))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:post content length exceeded max character limit"))
			})

			It("should return nil on success", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).To(BeNil())
			})
		})

		Context("issue post body check", func() {
			var ref = plumbing2.MakeIssueReference(1)

			It("should return error when 'labels' is not a list of strings", func() {
				fm := map[string]interface{}{"title": "title", "labels": "help,feature"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a list of string values"))
			})

			It("should return error when 'assignees' is not a list of strings", func() {
				fm := map[string]interface{}{"title": "title", "assignees": "help,feature"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a list of string values"))
			})

			It("should return error when 'labels' entries exceeded max", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:too many labels; cannot exceed 10"))
			})

			It("should return error when 'labels' entry type is not string", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{1}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a string list"))
			})

			It("should return error when 'labels' entry is not a valid label name", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{"&&bad_label"}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.labels, msg:invalid " +
					"identifier; only alphanumeric, _, and - characters are allowed"))
			})

			It("should return error when 'assignees' entries exceeded max", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:too many assignees; cannot exceed 10"))
			})

			It("should return error when 'assignees' entry type is not string", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{1}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a string list"))
			})

			It("should return error when 'assignees' entry is not a push key", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{"invalid_key"}}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.assignees, msg:invalid push key ID"))
			})
		})

		Context("merge request post body check", func() {
			var ref = plumbing2.MakeMergeRequestReference(1)

			It("should return error when 'base' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "base": 123}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.base, msg:expected a string value"))
			})

			It("should return error when 'baseHash' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "baseHash": 123}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.baseHash, msg:expected a string value"))
			})

			It("should return error when 'target' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "target": 123}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.target, msg:expected a string value"))
			})

			It("should return error when 'targetHash' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "targetHash": 123}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.targetHash, msg:expected a string value"))
			})

			It("should return error when 'base' branch is unset and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": ""}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.base, msg:base branch name is required"))
			})

			It("should return error when 'baseHash' is set but invalid and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "0x_invalid"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.baseHash, msg:base branch hash is not valid"))
			})

			It("should return error when 'target' branch is unset and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "target": ""}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.target, msg:target branch name is required"))
			})

			It("should return error when 'targetHash' is unsetand merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "target": "dev", "targetHash": ""}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.targetHash, msg:target branch hash is required"))
			})

			It("should return error when 'targetHash' is not valid and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "target": "dev", "targetHash": "0x_invalid"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.targetHash, msg:target branch hash is not valid"))
			})

			It("should return no error when successful", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "target": "dev", "targetHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310"}
				err := validation.CheckPostBody(nil, ref, wc, true, fm, []byte{1})
				Expect(err).To(BeNil())
			})

			When("merge request reference is not new", func() {
				It("should not return error when merge fields (base, baseHash, target, targetHash) are unset", func() {
					fm := map[string]interface{}{}
					ref := plumbing2.MakeMergeRequestReference(1)
					mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{
						mergerequest.MakeMergeRequestProposalID(1): {Outcome: state.ProposalOutcomeAccepted},
					}})
					err := validation.CheckPostBody(mockRepo, ref, wc, false, fm, []byte{1})
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".CheckMergeRequestPostBodyConsistency", func() {
		When("merge request reference is not new", func() {
			It("should return error when there is not merge proposal for the reference", func() {
				ref := plumbing2.MakeMergeRequestReference(1)
				mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{}})
				err := validation.CheckMergeRequestPostBodyConsistency(mockRepo, ref, false, map[string]interface{}{})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge request proposal not found"))
			})

			When("merge request proposal is finalized", func() {
				It("should return error when post body include merge request a field (base, baseHash, target, targetHash)", func() {
					ref := plumbing2.MakeMergeRequestReference(1)
					mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{
						mergerequest.MakeMergeRequestProposalID(1): {Outcome: state.ProposalOutcomeAccepted},
					}})
					err := validation.CheckMergeRequestPostBodyConsistency(mockRepo, ref, false, map[string]interface{}{"base": "master"})
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("cannot update 'base' field of a finalized merge request proposal"))
				})

				It("should return no error when post body does not contain merge request field", func() {
					ref := plumbing2.MakeMergeRequestReference(1)
					mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{
						mergerequest.MakeMergeRequestProposalID(1): {Outcome: state.ProposalOutcomeAccepted},
					}})
					err := validation.CheckMergeRequestPostBodyConsistency(mockRepo, ref, false, map[string]interface{}{})
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
