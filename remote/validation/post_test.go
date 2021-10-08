package validation_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/logic/contracts/mergerequest"
	"github.com/make-os/kit/mocks"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var mockKeepers *mocks.MockKeepers
	var commit *mocks.MockCommit

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		commit = mocks.NewMockCommit(ctrl)
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
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
			change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			detail := &types.TxDetail{Reference: "refs/heads/issues/1"}
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change, TxDetail: detail}
			mockRepo.EXPECT().HasMergeCommits(detail.Reference).Return(false, fmt.Errorf("error"))
			err := validation.ValidatePostCommit(mockRepo, nil, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check for merge commits in post reference: error"))
		})

		It("should return error when issue has merge commits", func() {
			change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
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
			change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			args := &validation.ValidatePostCommitArg{OldHash: "", Change: change, TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("check error")
				},
			}
			err := validation.ValidatePostCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("check error"))
		})

		When("target reference already exists", func() {
			It("should return error when unable to get ancestors", func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
				mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
				detail := &types.TxDetail{
					Reference: "refs/heads/issues/1",
				}

				mockRepoState := state.BareRepository()
				mockRepoState.References["refs/heads/issues/1"] = &state.Reference{Nonce: 1}
				mockRepo.EXPECT().GetState().Return(mockRepoState)

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
		})

		It("should return error when commit failed issue commit validation ", func() {
			change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			detail := &types.TxDetail{Reference: "refs/heads/issues/1"}

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
				mockRepoState = state.BareRepository()
			})

			It("should not return error when issue commit check is passed and issue checker func must be called once", func() {
				mockRepo.EXPECT().GetState().Return(mockRepoState)

				change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: &types.TxDetail{Reference: "refs/heads/issues/1"},
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						return &plumbing2.PostBody{}, nil
					},
				}
				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
			})

			When("ancestor commit post body included an admin field but the reference is new (does not already exist)", func() {
				It("should not return error", func() {
					mockRepo.EXPECT().GetState().Return(mockRepoState)

					change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
					detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
					args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
						TxDetail:    detail,
						CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error { return nil },
						CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
							return &plumbing2.PostBody{IssueFields: types.IssueFields{Labels: &[]string{"label_update"}}}, nil
						},
					}
					err := validation.ValidatePostCommit(mockRepo, commit, args)
					Expect(err).To(BeNil())
					Expect(detail.FlagCheckAdminUpdatePolicy).To(BeTrue())
				})
			})

			It("should populate tx detail reference data fields from post body", func() {
				mockRepo.EXPECT().GetState().Return(mockRepoState)

				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
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
							IssueFields: types.IssueFields{
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
				mockRepo.EXPECT().GetState().Return(mockRepoState)
				mockRepo.EXPECT().GetAncestors(commitObj, "", true).Return([]*object.Commit{}, nil)

				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{Hash: []byte("hash"), Data: &state.ReferenceData{Closed: true}}
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.PostBody{}, nil
					},
				}

				err := validation.ValidatePostCommit(mockRepo, commit, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrCannotWriteToClosedRef))
			})

			It("should return no error when issue reference has been previously closed and new issue commit set close=2", func() {
				mockRepo.EXPECT().GetState().Return(mockRepoState)
				mockRepo.EXPECT().GetAncestors(commitObj, "", true).Return([]*object.Commit{}, nil)

				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &types.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &types.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{Data: &state.ReferenceData{Closed: true}, Hash: []byte("hash")}
				args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
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
			})

			When("target reference already exist", func() {
				BeforeEach(func() {
					mockRepoState := state.BareRepository()
					mockRepoState.References["refs/heads/issues/1"] = &state.Reference{Nonce: 1, Data: &state.ReferenceData{}}
					mockRepo.EXPECT().GetState().Return(mockRepoState)
					mockRepo.EXPECT().GetAncestors(child, args.OldHash, true).Return([]*object.Commit{ancestor}, nil)
				})

				Specify("that post checker is called twice for both the commit and its ancestor", func() {
					change := &types.ItemChange{Item: &plumbing2.Obj{Data: child.Hash.String()}}
					callCount := 0
					args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
						TxDetail:    &types.TxDetail{Reference: "refs/heads/issues/1"},
						CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error { return nil },
						CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
							callCount++

							if callCount == 1 {
								Expect(commit.UnWrap()).To(Equal(ancestor))
								Expect(args.IsNew).To(BeFalse())
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

			When("target reference does not already exist", func() {
				BeforeEach(func() {
					mockRepoState := state.BareRepository()
					mockRepo.EXPECT().GetState().Return(mockRepoState)
				})

				Specify("that post checker is called once for only the commit", func() {
					change := &types.ItemChange{Item: &plumbing2.Obj{Data: child.Hash.String()}}
					callCount := 0
					args := &validation.ValidatePostCommitArg{OldHash: "", Change: change,
						TxDetail:    &types.TxDetail{Reference: "refs/heads/issues/1"},
						CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error { return nil },
						CheckPostCommit: func(r types.LocalRepo, commit types.Commit, args *validation.CheckPostCommitArgs) (*plumbing2.PostBody, error) {
							callCount++

							if callCount == 1 {
								Expect(commit.UnWrap()).To(Equal(child))
								Expect(args.IsNew).To(BeTrue())
							}

							Expect(args.Reference).To(Equal(detail.Reference))
							return &plumbing2.PostBody{}, nil
						},
					}
					err := validation.ValidatePostCommit(mockRepo, commit, args)
					Expect(err).To(BeNil())
					Expect(callCount).To(Equal(1))
				})
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
			commit.EXPECT().Tree().Return(nil, fmt.Errorf("bad query"))
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
			commit.EXPECT().Tree().Return(tree, nil)
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
			commit.EXPECT().Tree().Return(tree, nil)
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
			commit.EXPECT().Tree().Return(tree, nil)
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
			err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.field1","msg":"unexpected field"`))
		})

		It("should return error when an issue reference type is unknown", func() {
			ref := "refs/heads/unknown"
			err := validation.CheckPostBody(mockKeepers, nil, ref, repo.WrapCommit(commit), true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("unsupported post type"))
		})

		Context("common post body check", func() {
			var ref = plumbing2.MakeIssueReference(1)

			It("should return error when 'title' is not string", func() {
				fm := map[string]interface{}{"title": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.title","msg":"expected a string value"`))
			})

			It("should return error when 'replyTo' is not string", func() {
				fm := map[string]interface{}{"replyTo": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.replyTo","msg":"expected a string value"`))
			})

			It("should return error when 'reactions' is not string", func() {
				fm := map[string]interface{}{"reactions": "smile"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.reactions","msg":"expected a list of string values"`))
			})

			It("should return error when 'replyTo' is set in a new reference", func() {
				fm := map[string]interface{}{"replyTo": "0x123"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.replyTo","msg":"not expected in a new post commit"`))
			})

			It("should return error when 'title' is not provided in new reference", func() {
				fm := map[string]interface{}{}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.title","msg":"title is required"`))
			})

			It("should return error when 'title' exceeds max. characters", func() {
				fm := map[string]interface{}{"title": strings.Repeat("a", validation.MaxIssueTitleLen+1)}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.title","msg":"title is too long; cannot exceed .* characters"`))
			})

			It("should return error when 'replyTo' length is too low or too high", func() {
				fm := map[string]interface{}{"replyTo": "0x1"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.replyTo","msg":"invalid hash value"`))
				fm = map[string]interface{}{"replyTo": strings.Repeat("a", 41)}
				err = validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.replyTo","msg":"invalid hash value"`))
			})

			It("should return error when 'replyTo' hash does not point to an ancestor", func() {
				ancestor := "hash_of_ancestor"
				mockRepo.EXPECT().IsAncestor(ancestor, commit.Hash.String()).Return(fmt.Errorf("error"))
				fm := map[string]interface{}{"replyTo": "hash_of_ancestor"}
				err := validation.CheckPostBody(mockKeepers, mockRepo, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.replyTo","msg":"hash is not a known ancestor"`))
			})

			It("should return error when 'reaction' values exceed max", func() {
				fm := map[string]interface{}{"reactions": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.reactions","msg":"too many reactions; cannot exceed 10"`))
			})

			It("should return error when 'reaction' does not contain string entries", func() {
				fm := map[string]interface{}{"reactions": []interface{}{1}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.reactions","msg":"expected a string list"`))
			})

			It("should return error when 'reaction' includes an unknown reaction", func() {
				fm := map[string]interface{}{"reactions": []interface{}{"unknown_reaction"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, false, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.reactions","index":"0","msg":"reaction 'unknown_reaction' is unknown"`))
			})

			It("should return error when reference is new and 'content' is unset", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.content","msg":"post content is required"`))
			})

			It("should return error when reference is new and 'content' exceeds max", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, bytes.Repeat([]byte{1}, validation.MaxIssueContentLen+1))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.content","msg":"post content length exceeded max character limit"`))
			})

			It("should return nil on success", func() {
				fm := map[string]interface{}{"title": "a title"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).To(BeNil())
			})
		})

		Context("issue post body check", func() {
			var ref = plumbing2.MakeIssueReference(1)

			It("should return error when 'labels' is not a list of strings", func() {
				fm := map[string]interface{}{"title": "title", "labels": "help,feature"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.labels","msg":"expected a list of string values"`))
			})

			It("should return error when 'assignees' is not a list of strings", func() {
				fm := map[string]interface{}{"title": "title", "assignees": "help,feature"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.assignees","msg":"expected a list of string values"`))
			})

			It("should return error when 'labels' entries exceeded max", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.labels","msg":"too many labels; cannot exceed 10"`))
			})

			It("should return error when 'labels' entry type is not string", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{1}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.labels","msg":"expected a string list"`))
			})

			It("should return error when 'labels' entry is not a valid label name", func() {
				fm := map[string]interface{}{"title": "title", "labels": []interface{}{"&&bad_label"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.labels","index":"0","msg":"invalid ` +
					"identifier; only alphanumeric, _, and - characters are allowed"))
			})

			It("should return error when 'assignees' entries exceeded max", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.assignees","msg":"too many assignees; cannot exceed 10"`))
			})

			It("should return error when 'assignees' entry type is not string", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{1}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.assignees","msg":"expected a string list"`))
			})

			It("should return error when 'assignees' entry is not a push key", func() {
				fm := map[string]interface{}{"title": "title", "assignees": []interface{}{"invalid_key"}}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.assignees","index":"0","msg":"invalid push key ID"`))
			})
		})

		Context("merge request post body check", func() {
			var ref = plumbing2.MakeMergeRequestReference(1)

			It("should return error when 'base' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "base": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.base","msg":"expected a string value"`))
			})

			It("should return error when 'baseHash' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "baseHash": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.baseHash","msg":"expected a string value"`))
			})

			It("should return error when 'target' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "target": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.target","msg":"expected a string value"`))
			})

			It("should return error when 'targetHash' is not a string", func() {
				fm := map[string]interface{}{"title": "title", "targetHash": 123}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.targetHash","msg":"expected a string value"`))
			})

			It("should return error when 'base' branch is unset and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": ""}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.base","msg":"base branch name is required"`))
			})

			It("should return error when 'baseHash' is unset and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.baseHash","msg":"base branch hash is required"`))
			})

			It("should return error when 'baseHash' is set but invalid and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "0x_invalid"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.baseHash","msg":"base branch hash is not valid"`))
			})

			It("should return error when 'target' branch is unset and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310", "target": ""}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.target","msg":"target branch name is required"`))
			})

			It("should return error when 'targetHash' is unsetand merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310", "target": "dev", "targetHash": ""}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.targetHash","msg":"target branch hash is required"`))
			})

			It("should return error when 'targetHash' is not valid and merge request reference is new", func() {
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310", "target": "dev", "targetHash": "0x_invalid"}
				err := validation.CheckPostBody(mockKeepers, nil, ref, wc, true, fm, []byte{1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(`"field":"<commit#.*>.targetHash","msg":"target branch hash is not valid"`))
			})

			It("should return no error when successful", func() {
				repoState := state.BareRepository()
				repoState.References["refs/heads/master"] = &state.Reference{Hash: util.MustFromHex("7f92315bdc59a859aefd0d932173cd00fd1ec310")}
				repoState.References["refs/heads/dev"] = &state.Reference{Hash: util.MustFromHex("519cca1e9aad6dda3db6b7c1b31a7d733a199ef4")}
				mockRepo.EXPECT().GetState().Return(repoState)
				fm := map[string]interface{}{"title": "title", "base": "master", "baseHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310", "target": "dev", "targetHash": "519cca1e9aad6dda3db6b7c1b31a7d733a199ef4"}
				err := validation.CheckPostBody(mockKeepers, mockRepo, ref, wc, true, fm, []byte{1})
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckMergeRequestPostBodyConsistency", func() {
		When("merge request reference is not new", func() {
			It("should return error when there is not merge proposal for the reference", func() {
				ref := plumbing2.MakeMergeRequestReference(1)
				mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{}})
				err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, false, map[string]interface{}{})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge request proposal not found"))
			})

			When("merge request proposal is finalized", func() {
				It("should return error when post body include merge request a field (base, baseHash, target, targetHash)", func() {
					ref := plumbing2.MakeMergeRequestReference(1)
					mockRepo.EXPECT().GetState().Return(&state.Repository{Proposals: map[string]*state.RepoProposal{
						mergerequest.MakeMergeRequestProposalID(1): {Outcome: state.ProposalOutcomeAccepted},
					}})
					err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, false, map[string]interface{}{"base": "master"})
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("cannot update 'base' field of a finalized merge request proposal"))
				})
			})
		})

		It("should return error if base branch does not exist as a reference", func() {
			repoState := &state.Repository{}
			mockRepo.EXPECT().GetState().Return(repoState)
			ref := plumbing2.MakeMergeRequestReference(1)
			err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
				"base": "major",
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("base branch (major) is unknown"))
		})

		It("should return error if base branch hash is set but does not match base reference hash on repo state", func() {
			repoState := &state.Repository{References: map[string]*state.Reference{"refs/heads/major": {Hash: util.MustFromHex("519cca1e9aad6dda3db6b7c1b31a7d733a199ef4")}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			ref := plumbing2.MakeMergeRequestReference(1)
			err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
				"base":     "major",
				"baseHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310",
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("base branch (major) hash does not match upstream state"))
		})

		It("should return error if target branch does not exist as a reference", func() {
			repoState := &state.Repository{}
			mockRepo.EXPECT().GetState().Return(repoState)
			ref := plumbing2.MakeMergeRequestReference(1)
			err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
				"target": "dev",
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target branch (dev) is unknown"))
		})

		When("target branch has a path (/repo/branch)", func() {
			It("should return error if repo does not exist", func() {
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				repo1 := state.BareRepository()
				mockRepo.EXPECT().GetState().Return(repo1)
				mockRepoKeeper.EXPECT().GetNoPopulate("repo1").Return(repo1)
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				ref := plumbing2.MakeMergeRequestReference(1)
				err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
					"target": "/repo1/dev",
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target branch's repository (repo1) does not exist"))
			})

			It("should return error if repo exists but the branch does not exist", func() {
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				repo1 := state.BareRepository()
				repo1.Balance = "20.3"
				mockRepo.EXPECT().GetState().Return(repo1)
				mockRepoKeeper.EXPECT().GetNoPopulate("repo1").Return(repo1)
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				ref := plumbing2.MakeMergeRequestReference(1)
				err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
					"target": "/repo1/dev",
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target branch (dev) of (repo1) is unknown"))
			})

			It("should return no error if repo exists and the branch also exists", func() {
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				repo1 := state.BareRepository()
				repo1.References = map[string]*state.Reference{"refs/heads/dev/testing": {}}
				mockRepo.EXPECT().GetState().Return(repo1)
				mockRepoKeeper.EXPECT().GetNoPopulate("repo1").Return(repo1)
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				ref := plumbing2.MakeMergeRequestReference(1)
				err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
					"target": "/repo1/dev/testing",
				})
				Expect(err).To(BeNil())
			})

			It("should return error if target branch hash is set but does not match target reference hash on repo state", func() {
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				repo1 := state.BareRepository()
				repo1.References = map[string]*state.Reference{"refs/heads/dev/testing": {Hash: util.MustFromHex("519cca1e9aad6dda3db6b7c1b31a7d733a199ef4")}}
				mockRepo.EXPECT().GetState().Return(repo1)
				mockRepoKeeper.EXPECT().GetNoPopulate("repo1").Return(repo1)
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				ref := plumbing2.MakeMergeRequestReference(1)
				err := validation.CheckMergeRequestPostBodyConsistency(mockKeepers, mockRepo, ref, true, map[string]interface{}{
					"target":     "/repo1/dev/testing",
					"targetHash": "7f92315bdc59a859aefd0d932173cd00fd1ec310",
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target branch (dev/testing) hash does not match upstream state"))
			})
		})
	})
})
