package dht_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/testutil"
	types2 "gitlab.com/makeos/mosdef/types"
	io2 "gitlab.com/makeos/mosdef/util/io"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
	packfile2 "gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type fakePackfile struct {
	name string
}

func (f *fakePackfile) Read(p []byte) (n int, err error) {
	return
}

func (f *fakePackfile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (f *fakePackfile) Close() error {
	return nil
}

var _ = Describe("BasicCommitStreamer", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockHost *mocks.MockHost
	var mockDHT *mocks.MockDHT
	var cs *dht.BasicCommitStreamer

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockHost = mocks.NewMockHost(ctrl)
		mockDHT = mocks.NewMockDHT(ctrl)
	})

	BeforeEach(func() {
		mockHost.EXPECT().SetStreamHandler(gomock.Any(), gomock.Any())
		mockDHT.EXPECT().Host().Return(mockHost)
		cs = dht.NewCommitStreamer(mockDHT, cfg)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".NewCommitStreamer", func() {
		It("should register commit stream protocol handler", func() {
			mockHost.EXPECT().SetStreamHandler(dht.CommitStreamProtocolID, gomock.Any())
			mockDHT.EXPECT().Host().Return(mockHost)
			dht.NewCommitStreamer(mockDHT, cfg)
		})
	})

	Describe(".Announce", func() {
		It("should announce commit hash", func() {
			hash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"
			ctx := context.Background()
			mockDHT.EXPECT().Announce(ctx, dht.MakeCommitKey([]byte(hash)))
			err := cs.Announce(ctx, []byte(hash))
			Expect(err).To(BeNil())
		})

		It("should return error when announce attempt failed", func() {
			hash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"
			ctx := context.Background()
			mockDHT.EXPECT().Announce(ctx, dht.MakeCommitKey([]byte(hash))).Return(fmt.Errorf("error"))
			err := cs.Announce(ctx, []byte(hash))
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})

	Describe(".OnRequest", func() {
		It("should return error when unable to read stream", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return 0, fmt.Errorf("read error")
			})
			err := cs.OnRequest(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read request: read error"))
		})

		It("should return ErrUnknownMsgType when message type is unknown", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				msg := []byte("unknown")
				copy(p, msg)
				return len(msg), nil
			})
			err := cs.OnRequest(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(dht.ErrUnknownMsgType))
		})

		It("should call 'Want' handler when message is MsgTypeWant", func() {
			msg := []byte(dht.MsgTypeWant)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, msg)
				return len(msg), nil
			})
			cs.OnWantHandler = func(m []byte, s network.Stream) error {
				Expect(msg).To(Equal(msg))
				return nil
			}
			err := cs.OnRequest(mockStream)
			Expect(err).To(BeNil())
		})

		It("should call 'Send' handler when message is MsgTypeSend", func() {
			msg := []byte(dht.MsgTypeSend)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, msg)
				return len(msg), nil
			})
			cs.OnSendHandler = func(m []byte, s network.Stream) error {
				Expect(msg).To(Equal(msg))
				return nil
			}
			err := cs.OnRequest(mockStream)
			Expect(err).To(BeNil())
		})
	})

	Describe(".OnWant", func() {
		hash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"

		It("should return error if msg could not be parsed", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			err := cs.OnWant([]byte(""), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("malformed message"))
		})

		It("should return error if unable to get local repository", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return nil, fmt.Errorf("failed to get repo")
			}
			err := cs.OnWant(dht.MakeWantMsg("repo1", []byte(hash)), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo"))
		})

		It("should return error if extracted commit key is malformed", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return mockRepo, nil
			}
			err := cs.OnWant(dht.MakeWantMsg("repo1", []byte(hash)), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("malformed commit key"))
		})

		It("should return error when non-ErrObjectNotFound is returned when getting commit from local repo", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, fmt.Errorf("unexpected error"))
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return mockRepo, nil
			}
			key := dht.MakeCommitKey([]byte(hash))
			err := cs.OnWant(dht.MakeWantMsg("repo1", key), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unexpected error"))
		})

		When("ErrObjectNotFound is returned when getting commit from local repo", func() {
			It("should return error when writing a 'NOPE' response failed", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockStream.EXPECT().Reset()
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, plumbing2.ErrObjectNotFound)
				mockStream.EXPECT().Write(dht.MakeNopeMsg()).Return(0, fmt.Errorf("write error"))
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnWant(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write 'nope' message: write error"))
			})
		})

		When("commit object exist in local repo", func() {
			It("should return error when writing 'HAVE' response failed", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockStream.EXPECT().Reset()
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, nil)
				mockStream.EXPECT().Write(dht.MakeHaveMsg()).Return(0, fmt.Errorf("write error"))
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnWant(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("write error"))
			})

			It("should return no error when writing 'HAVE' response succeeds", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, nil)
				mockStream.EXPECT().Write(dht.MakeHaveMsg()).Return(0, nil)
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnWant(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".OnSend", func() {
		hash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"

		It("should return error if msg could not be parsed", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			err := cs.OnSend([]byte(""), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("malformed message"))
		})

		It("should return error if unable to get local repository", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return nil, fmt.Errorf("failed to get repo")
			}
			err := cs.OnSend(dht.MakeWantMsg("repo1", []byte(hash)), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo"))
		})

		It("should return error if extracted commit key is malformed", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return mockRepo, nil
			}
			err := cs.OnSend(dht.MakeWantMsg("repo1", []byte(hash)), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("malformed commit key"))
		})

		It("should return error when non-ErrObjectNotFound is returned when getting commit from local repo", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, fmt.Errorf("unexpected error"))
			cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
				return mockRepo, nil
			}
			key := dht.MakeCommitKey([]byte(hash))
			err := cs.OnSend(dht.MakeWantMsg("repo1", key), mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unexpected error"))
		})

		When("ErrObjectNotFound is returned when getting commit from local repo", func() {
			It("should return error when writing a 'NOPE' response failed", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockStream.EXPECT().Reset()
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, plumbing2.ErrObjectNotFound)
				mockStream.EXPECT().Write(dht.MakeNopeMsg()).Return(0, fmt.Errorf("write error"))
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnSend(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write 'nope' message: write error"))
			})
		})

		When("commit object exist in local repo", func() {
			It("should return error when generating a packfile for the commit failed", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockStream.EXPECT().Reset()
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, nil)
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				cs.PackCommit = func(repo types.LocalRepo, commit *object.Commit) (io.Reader, error) {
					return nil, fmt.Errorf("error")
				}
				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnSend(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to generate commit packfile: error"))
			})

			It("should return no error", func() {
				mockStream := mocks.NewMockStream(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().CommitObject(plumbing2.NewHash(hash)).Return(nil, nil)
				cs.RepoGetter = func(string, string) (types.LocalRepo, error) {
					return mockRepo, nil
				}
				cs.PackCommit = func(repo types.LocalRepo, commit *object.Commit) (io.Reader, error) {
					return bytes.NewReader(nil), nil
				}
				mockStream.EXPECT().Close()

				key := dht.MakeCommitKey([]byte(hash))
				err := cs.OnSend(dht.MakeWantMsg("repo1", key), mockStream)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".Get", func() {
		var ctx = context.Background()
		var repoName = "repo1"
		var hash = []byte("6fe5e981f7defdfb907c1237e2e8427696adafa7")

		It("should return error when unable to get providers", func() {
			mockDHT.EXPECT().GetProviders(ctx, dht.MakeCommitKey(hash)).Return(nil, fmt.Errorf("error"))
			_, _, err := cs.Get(ctx, repoName, hash)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get providers: error"))
		})

		It("should return ErrNoProviderFound when no provider is found", func() {
			mockDHT.EXPECT().GetProviders(ctx, dht.MakeCommitKey(hash)).Return(nil, nil)
			_, _, err := cs.Get(ctx, repoName, hash)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(dht.ErrNoProviderFound))
		})

		It("should return error when request failed", func() {
			mockDHT.EXPECT().Host().Return(mockHost)

			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}
			mockDHT.EXPECT().GetProviders(ctx, dht.MakeCommitKey(hash)).Return([]peer.AddrInfo{prov}, nil)
			mockComReq := mocks.NewMockCommitRequester(ctrl)
			mockComReq.EXPECT().Do(ctx).Return(nil, fmt.Errorf("request error"))
			cs.MakeRequester = func(args dht.CommitRequesterArgs) dht.CommitRequester {
				return mockComReq
			}
			_, _, err := cs.Get(ctx, repoName, hash)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("request failed: request error"))
		})

		It("should return error when packfile failed validation", func() {
			mockDHT.EXPECT().Host().Return(mockHost)

			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}
			mockDHT.EXPECT().GetProviders(ctx, dht.MakeCommitKey(hash)).Return([]peer.AddrInfo{prov}, nil)
			mockComReq := mocks.NewMockCommitRequester(ctrl)
			mockComReq.EXPECT().Do(ctx).Return(nil, nil)
			cs.MakeRequester = func(args dht.CommitRequesterArgs) dht.CommitRequester {
				return mockComReq
			}
			cs.ValidatePackfile = func(hash []byte, pack io2.ReadSeekerCloser) (targetCommit *object.Commit, err error) {
				return nil, fmt.Errorf("validation error")
			}
			_, _, err := cs.Get(ctx, repoName, hash)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("validation failed: validation error"))
		})

		It("should return packfile on success", func() {
			mockDHT.EXPECT().Host().Return(mockHost)

			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}
			mockDHT.EXPECT().GetProviders(ctx, dht.MakeCommitKey(hash)).Return([]peer.AddrInfo{prov}, nil)
			mockComReq := mocks.NewMockCommitRequester(ctrl)

			pack, err := ioutil.TempFile(os.TempDir(), "")
			Expect(err).To(BeNil())
			defer pack.Close()
			mockComReq.EXPECT().Do(ctx).Return(pack, nil)
			cs.MakeRequester = func(args dht.CommitRequesterArgs) dht.CommitRequester {
				return mockComReq
			}

			cs.ValidatePackfile = func(hash []byte, pack io2.ReadSeekerCloser) (targetCommit *object.Commit, err error) {
				return nil, nil
			}
			res, _, err := cs.Get(ctx, repoName, hash)
			Expect(err).To(BeNil())
			Expect(res).To(Equal(pack))
		})
	})

	Describe(".GetAncestors", func() {
		var ctx = context.Background()
		var repoName = "repo1"
		var hash = []byte("6fe5e981f7defdfb907c1237e2e8427696adafa7")

		It("should return error when unable to get target repository", func() {
			cs := mocks.NewMockCommitStreamer(ctrl)
			_, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
				return nil, fmt.Errorf("error")
			}, dht.GetAncestorArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo: error"))
		})

		When("end commit hash is provided", func() {
			It("should return error if end commit object does not exist locally", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().ObjectExist(string(hash)).Return(false)
				_, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					EndCommitHash: hash,
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("end commit must already exist in the local repo"))
			})
		})

		It("should return error if end unable to get start hash from DHT", func() {
			cs := mocks.NewMockCommitStreamer(ctrl)
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			cs.EXPECT().Get(ctx, repoName, hash).Return(nil, nil, fmt.Errorf("error"))

			_, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
				return mockRepo, nil
			}, dht.GetAncestorArgs{
				StartCommitHash: hash,
				RepoName:        repoName,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("start commit hash and end commit hash match", func() {
			It("should return start commit pack file when ExcludeEndCommit is false", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommitPackfile := &fakePackfile{"pack-1"}
				mockRepo.EXPECT().ObjectExist(string(hash)).Return(true)
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash:  hash,
					EndCommitHash:    hash,
					RepoName:         repoName,
					ExcludeEndCommit: false,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(1))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
			})

			It("should not return start commit pack file when ExcludeEndCommit is true", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommitPackfile := &fakePackfile{"pack-1"}
				mockRepo.EXPECT().ObjectExist(string(hash)).Return(true)
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash:  hash,
					EndCommitHash:    hash,
					RepoName:         repoName,
					ExcludeEndCommit: true,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(0))
			})
		})

		When("start commit does not have parents", func() {
			It("should return start commit pack file alone", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(1))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
			})
		})

		When("start commit has one parent", func() {
			var parentHash = []byte("7a561e23f4e81c61df1b0dc63a89ae9c8d5680cd")

			It("should return start commit and its parent commit packfiles", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)

				parentCommit := &object.Commit{Hash: plumbing2.NewHash(string(parentHash))}
				parentCommitPackfile := &fakePackfile{"pack-2"}
				cs.EXPECT().Get(ctx, repoName, parentHash).Return(parentCommitPackfile, parentCommit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(2))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
				Expect(packfiles[1]).To(Equal(parentCommitPackfile))
			})

			It("should not add parent to wantlist if it already exists", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(1))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
			})

			It("should not add parent to wantlist if parent is the end commit and ExcludeEndCommit=true", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash:  hash,
					RepoName:         repoName,
					EndCommitHash:    parentHash,
					ExcludeEndCommit: true,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(1))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
			})

			It("should add parent to wantlist if parent is the end commit, parent already exist locally and ExcludeEndCommit=false", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)

				parentCommit := &object.Commit{Hash: plumbing2.NewHash(string(parentHash))}
				parentCommitPackfile := &fakePackfile{"pack-2"}
				cs.EXPECT().Get(ctx, repoName, parentHash).Return(parentCommitPackfile, parentCommit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash:  hash,
					RepoName:         repoName,
					EndCommitHash:    parentHash,
					ExcludeEndCommit: false,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(2))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
				Expect(packfiles[1]).To(Equal(parentCommitPackfile))
			})
		})

		When("start commit has two parents", func() {
			var parentHash = []byte("7a561e23f4e81c61df1b0dc63a89ae9c8d5680cd")
			var parent2Hash = []byte("c988dcc9fc47958626c8bd1b956817e5b5bb0105")

			It("should return start commit and its parents commit packfiles", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parent2Hash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)
				mockRepo.EXPECT().ObjectExist(string(parent2Hash)).Return(false)

				parentCommit := &object.Commit{Hash: plumbing2.NewHash(string(parentHash))}
				parentCommitPackfile := &fakePackfile{"pack-2"}
				cs.EXPECT().Get(ctx, repoName, parentHash).Return(parentCommitPackfile, parentCommit, nil)

				parent2Commit := &object.Commit{Hash: plumbing2.NewHash(string(parent2Hash))}
				parent2CommitPackfile := &fakePackfile{"pack-3"}
				cs.EXPECT().Get(ctx, repoName, parent2Hash).Return(parent2CommitPackfile, parent2Commit, nil)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(3))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
				Expect(packfiles[1]).To(Equal(parentCommitPackfile))
				Expect(packfiles[2]).To(Equal(parent2CommitPackfile))
			})
		})

		Context("when end commit has been fetched/seen but wantlist is not empty", func() {
			var parentHash = []byte("7a561e23f4e81c61df1b0dc63a89ae9c8d5680cd")
			var parent2Hash = []byte("c988dcc9fc47958626c8bd1b956817e5b5bb0105")

			It("should return add object to wantlist if ErrObjectNotFound is returned while performing ancestor check", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parent2Hash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)
				mockRepo.EXPECT().ObjectExist(string(parent2Hash)).Return(false)

				parentCommit := &object.Commit{Hash: plumbing2.NewHash(string(parentHash))}
				parentCommitPackfile := &fakePackfile{"pack-2"}
				cs.EXPECT().Get(ctx, repoName, parentHash).Return(parentCommitPackfile, parentCommit, nil)

				parent2Commit := &object.Commit{Hash: plumbing2.NewHash(string(parent2Hash))}
				parent2CommitPackfile := &fakePackfile{"pack-3"}
				cs.EXPECT().Get(ctx, repoName, parent2Hash).Return(parent2CommitPackfile, parent2Commit, nil)

				mockRepo.EXPECT().IsAncestor(string(parent2Hash), string(parentHash)).Return(plumbing2.ErrObjectNotFound)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
					EndCommitHash:   parentHash,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(3))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
				Expect(packfiles[1]).To(Equal(parentCommitPackfile))
				Expect(packfiles[2]).To(Equal(parent2CommitPackfile))
			})

			It("should return add object to wantlist if ErrNotAnAncestor is returned while performing ancestor check", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parent2Hash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)
				mockRepo.EXPECT().ObjectExist(string(parent2Hash)).Return(false)

				parentCommit := &object.Commit{Hash: plumbing2.NewHash(string(parentHash))}
				parentCommitPackfile := &fakePackfile{"pack-2"}
				cs.EXPECT().Get(ctx, repoName, parentHash).Return(parentCommitPackfile, parentCommit, nil)

				parent2Commit := &object.Commit{Hash: plumbing2.NewHash(string(parent2Hash))}
				parent2CommitPackfile := &fakePackfile{"pack-3"}
				cs.EXPECT().Get(ctx, repoName, parent2Hash).Return(parent2CommitPackfile, parent2Commit, nil)

				mockRepo.EXPECT().IsAncestor(string(parent2Hash), string(parentHash)).Return(repo.ErrNotAnAncestor)

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
					EndCommitHash:   parentHash,
				})
				Expect(err).To(BeNil())
				Expect(packfiles).To(HaveLen(3))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
				Expect(packfiles[1]).To(Equal(parentCommitPackfile))
				Expect(packfiles[2]).To(Equal(parent2CommitPackfile))
			})

			It("should return error and current packfiles result if a non-ErrNotAnAncestor is returned while performing ancestor check", func() {
				cs := mocks.NewMockCommitStreamer(ctrl)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
				startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parent2Hash)))
				startCommitPackfile := &fakePackfile{"pack-1"}
				cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(true)
				mockRepo.EXPECT().ObjectExist(string(parentHash)).Return(false)
				mockRepo.EXPECT().ObjectExist(string(parent2Hash)).Return(false)

				mockRepo.EXPECT().IsAncestor(string(parent2Hash), string(parentHash)).Return(fmt.Errorf("bad error"))

				packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
					return mockRepo, nil
				}, dht.GetAncestorArgs{
					StartCommitHash: hash,
					RepoName:        repoName,
					EndCommitHash:   parentHash,
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to perform ancestor check: bad error"))
				Expect(packfiles).To(HaveLen(1))
				Expect(packfiles[0]).To(Equal(startCommitPackfile))
			})
		})

		Context("use callback to collect result, instead method returned result", func() {
			var parentHash = []byte("7a561e23f4e81c61df1b0dc63a89ae9c8d5680cd")

			When("ResultCB is provided", func() {
				It("should pass result to the callback and zero packfiles must be returned from the method", func() {
					cs := mocks.NewMockCommitStreamer(ctrl)
					mockRepo := mocks.NewMockLocalRepo(ctrl)
					startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
					startCommitPackfile := &fakePackfile{"pack-1"}
					cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

					cbPackfiles := []io2.ReadSeekerCloser{}
					packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
						return mockRepo, nil
					}, dht.GetAncestorArgs{
						StartCommitHash: hash,
						RepoName:        repoName,
						ResultCB: func(packfile io2.ReadSeekerCloser) error {
							cbPackfiles = append(cbPackfiles, packfile)
							return nil
						},
					})
					Expect(err).To(BeNil())
					Expect(packfiles).To(HaveLen(0))
					Expect(cbPackfiles).To(HaveLen(1))
					Expect(cbPackfiles[0]).To(Equal(startCommitPackfile))
				})
			})

			When("callback returns non-ErrExit error", func() {
				It("should return start commit and its parent commit packfiles", func() {
					cs := mocks.NewMockCommitStreamer(ctrl)
					mockRepo := mocks.NewMockLocalRepo(ctrl)
					startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
					startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
					startCommitPackfile := &fakePackfile{"pack-1"}
					cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

					cbPackfiles := []io2.ReadSeekerCloser{}
					packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
						return mockRepo, nil
					}, dht.GetAncestorArgs{
						StartCommitHash: hash,
						RepoName:        repoName,
						ResultCB: func(packfile io2.ReadSeekerCloser) error {
							cbPackfiles = append(cbPackfiles, packfile)
							return fmt.Errorf("error")
						},
					})
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("error"))
					Expect(packfiles).To(HaveLen(0))
					Expect(cbPackfiles).To(HaveLen(1))
					Expect(cbPackfiles[0]).To(Equal(startCommitPackfile))
				})
			})

			When("callback returns ErrExit error", func() {
				It("should return start commit and its parent commit packfiles", func() {
					cs := mocks.NewMockCommitStreamer(ctrl)
					mockRepo := mocks.NewMockLocalRepo(ctrl)
					startCommit := &object.Commit{Hash: plumbing2.NewHash(string(hash))}
					startCommit.ParentHashes = append(startCommit.ParentHashes, plumbing2.NewHash(string(parentHash)))
					startCommitPackfile := &fakePackfile{"pack-1"}
					cs.EXPECT().Get(ctx, repoName, hash).Return(startCommitPackfile, startCommit, nil)

					cbPackfiles := []io2.ReadSeekerCloser{}
					packfiles, err := dht.GetAncestors(ctx, cs, func(gitBinPath, path string) (types.LocalRepo, error) {
						return mockRepo, nil
					}, dht.GetAncestorArgs{
						StartCommitHash: hash,
						RepoName:        repoName,
						ResultCB: func(packfile io2.ReadSeekerCloser) error {
							cbPackfiles = append(cbPackfiles, packfile)
							return types2.ErrExit
						},
					})
					Expect(err).To(BeNil())
					Expect(packfiles).To(HaveLen(0))
					Expect(cbPackfiles).To(HaveLen(1))
					Expect(cbPackfiles[0]).To(Equal(startCommitPackfile))
				})
			})
		})
	})

	Describe(".Validate", func() {
		var hash = []byte("7a561e23f4e81c61df1b0dc63a89ae9c8d5680cd")

		It("should return error when packfile did not contain the target commit", func() {
			packfile := &fakePackfile{}
			cs.Unpack = func(pack io.ReadSeeker, cb plumbing.UnpackCallback) (err error) {
				cb(&packfile2.ObjectHeader{}, func() (object.Object, error) { return &object.Tree{}, nil })
				return nil
			}
			_, err := cs.Validate(hash, packfile)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target commit was not found in packfile"))
		})

		It("should return error when packfile did not contain the target commit tree", func() {
			treeHash := "6081bfcf869e310ed06304641fdf7c365a03ac56"
			packfile := &fakePackfile{}
			cs.Unpack = func(pack io.ReadSeeker, cb plumbing.UnpackCallback) (err error) {
				cb(&packfile2.ObjectHeader{Type: plumbing2.CommitObject}, func() (object.Object, error) {
					return &object.Commit{Hash: plumbing2.NewHash(string(hash)), TreeHash: plumbing2.NewHash(treeHash)}, nil
				})
				return nil
			}
			_, err := cs.Validate(hash, packfile)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target commit tree was not found in packfile"))
		})

		It("should return error when packfile did not contain a tree entry", func() {
			treeHash := "6081bfcf869e310ed06304641fdf7c365a03ac56"
			entryHash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"
			packfile := &fakePackfile{}
			cs.Unpack = func(pack io.ReadSeeker, cb plumbing.UnpackCallback) (err error) {
				cb(&packfile2.ObjectHeader{Type: plumbing2.CommitObject}, func() (object.Object, error) {
					return &object.Commit{Hash: plumbing2.NewHash(string(hash)), TreeHash: plumbing2.NewHash(treeHash)}, nil
				})
				cb(&packfile2.ObjectHeader{Type: plumbing2.TreeObject}, func() (object.Object, error) {
					return &object.Tree{
						Hash:    plumbing2.NewHash(treeHash),
						Entries: []object.TreeEntry{{Name: "file", Hash: plumbing2.NewHash(entryHash)}},
					}, nil
				})
				return nil
			}
			_, err := cs.Validate(hash, packfile)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target commit tree entry (file) was not found in packfile"))
		})

		It("should return no error", func() {
			treeHash := "6081bfcf869e310ed06304641fdf7c365a03ac56"
			entryHash := "6fe5e981f7defdfb907c1237e2e8427696adafa7"
			packfile := &fakePackfile{}
			cs.Unpack = func(pack io.ReadSeeker, cb plumbing.UnpackCallback) (err error) {
				cb(&packfile2.ObjectHeader{Type: plumbing2.CommitObject}, func() (object.Object, error) {
					return &object.Commit{Hash: plumbing2.NewHash(string(hash)), TreeHash: plumbing2.NewHash(treeHash)}, nil
				})
				cb(&packfile2.ObjectHeader{Type: plumbing2.TreeObject}, func() (object.Object, error) {
					return &object.Tree{
						Hash:    plumbing2.NewHash(treeHash),
						Entries: []object.TreeEntry{{Name: "file", Hash: plumbing2.NewHash(entryHash)}},
					}, nil
				})
				cb(&packfile2.ObjectHeader{Type: plumbing2.BlobObject}, func() (object.Object, error) {
					return &object.Blob{Hash: plumbing2.NewHash(entryHash)}, nil
				})
				return nil
			}
			_, err := cs.Validate(hash, packfile)
			Expect(err).To(BeNil())
		})
	})
})
