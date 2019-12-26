package repo

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types/mocks"

	"github.com/bitfield/script"
	"golang.org/x/crypto/openpgp"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var repo types.BareRepo
	var path string
	var gpgKeyID string
	var pubKey string
	var privKey *openpgp.Entity
	var privKey2 *openpgp.Entity
	var ctrl *gomock.Controller

	var gpgPubKeyGetter = func(pkId string) (string, error) {
		return pubKey, nil
	}

	var gpgInvalidPubKeyGetter = func(pkId string) (string, error) {
		return "invalid key", nil
	}

	var gpgPubKeyGetterWithErr = func(err error) func(pkId string) (string, error) {
		return func(pkId string) (string, error) {
			return "", err
		}
	}

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		ctrl = gomock.NewController(GinkgoT())

		cfg.Node.GitBinPath = "/usr/bin/git"
		gpgKeyID = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		gpgKeyID2 := testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey, err = crypto.GetGPGPublicKeyStr(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey, err = crypto.GetGPGPrivateKey(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey2, err = crypto.GetGPGPrivateKey(gpgKeyID2, testutil.GPGProgramPath, cfg.DataDir())
		Expect(err).To(BeNil())
		GitEnv = append(GitEnv, "GNUPGHOME="+cfg.DataDir())
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".checkCommit", func() {
		var cob *object.Commit
		var err error

		When("txline is not set", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='txline was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txline was not set"))
			})
		})

		When("txline.pkId is not valid", func() {
			BeforeEach(func() {
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", "invalid_pk_id")
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("commit is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the commit with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the commit with your gpg key"))
			})
		})

		When("commit is signed but unable to get public key using the pkID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("commit has a signature and the signature is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("txline is not set", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='txline was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txline was not set"))
			})
		})

		When("txline.pkId is not valid", func() {
			BeforeEach(func() {
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", "invalid_pk_id")
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("tag is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the tag with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the tag with your gpg key"))
			})
		})

		When("tag is signed but unable to get public key using the pkID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("tag has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("tag has a valid signature but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err=''", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*referenced commit.* is unsigned.*"))
			})
		})

		When("tag has a valid signature and the referenced commit is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkNote", func() {
		var err error

		When("target note does not exist", func() {
			BeforeEach(func() {
				_, err = checkNote(repo, "unknown", gpgPubKeyGetter)
			})

			It("should return err='unable to fetch note entries (unknown)'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to fetch note entries (unknown)"))
			})
		})

		When("a note does not have a tx blob object", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='unacceptable note. it does not have a signed transaction object'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unacceptable note. it does not have a signed transaction object"))
			})
		})

		When("a notes tx blob has invalid tx line", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx:invalid")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='note (refs/notes/note1): txline is malformed'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1): txline is malformed"))
			})
		})

		When("a notes tx blob has invalid signature format", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=xyz")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='note (refs/notes/note1): field:sig, msg: signature format is not valid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1): field:sig, msg: signature format is not valid"))
			})
		})

		When("a notes tx blob has an unknown public key id", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetterWithErr(fmt.Errorf("error finding pub key")))
			})

			It("should return err='unable to verify note (refs/notes/note1). public key was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key was not found"))
			})
		})

		When("a notes tx blob includes a public key id to an invalid public key", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgInvalidPubKeyGetter)
			})

			It("should return err='unable to verify note (refs/notes/note1). public key .. was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key is not valid"))
			})
		})

		When("a note's signature is not signed with an expected private key", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000000" + commitHash)
				sig, err = crypto.GPGSign(privKey2, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='invalid signature: RSA verification failure'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: RSA verification failure"))
			})
		})

		When("a note's signature message content/format is not expected", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000001" + commitHash)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='...invalid signature: hash tag doesn't match'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: hash tag doesn't match"))
			})
		})

		When("a note's signature is valid", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000000" + commitHash)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".validateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get tag object: tag not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get tag object: tag not found"))
			})
		})

		When("branch item points to a valid commit", func() {
			var cob *object.Commit
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))

				change := &types.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: cob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("annotated tag item points to a valid commit", func() {
			var tob *object.Tag
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				change := &types.ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("lightweight tag item points to a valid commit", func() {
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndLightWeightTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")

				change := &types.ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tagRef.Target().String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkPushNoteSyntax", func() {
		key := crypto.NewKeyFromIntSeed(1)
		okTx := &PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().Base58()}
		bz, _ := key.PrivKey().Sign(okTx.Bytes())
		okTx.NodeSig = bz

		var cases = [][]interface{}{
			{&PushNote{}, fmt.Errorf("field:repoName, error:repo name is required")},
			{&PushNote{RepoName: "repo"}, fmt.Errorf("field:pusherKeyId, error:pusher gpg key id is required")},
			{&PushNote{RepoName: "repo", PusherKeyID: "xyz"}, fmt.Errorf("field:pusherKeyId, error:pusher gpg key is not valid")},
			{&PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42)}, fmt.Errorf("field:timestamp, error:timestamp is too old")},
			{&PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42), Timestamp: time.Now().Unix()}, fmt.Errorf("field:nodePubKey, error:push node public key is required")},
			{&PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42), Timestamp: time.Now().Unix(), NodePubKey: "key"}, fmt.Errorf("field:nodePubKey, error:push node public key is not valid")},
			{&PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().Base58()}, fmt.Errorf("field:nodeSig, error:push node signature is required")},
			{&PushNote{RepoName: "repo", PusherKeyID: strings.Repeat("x", 42), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().Base58(), NodeSig: []byte("invalid")}, fmt.Errorf("field:nodeSig, error:failed to verify signature with public key")},
			{okTx, nil},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{
				{},
			}}, fmt.Errorf("index:0, field:references.name, error:name is required")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref"}}}, fmt.Errorf("index:0, field:references.oldHash, error:old hash is required")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: "abc"}}}, fmt.Errorf("index:0, field:references.oldHash, error:old hash is not valid")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40)}}}, fmt.Errorf("index:0, field:references.newHash, error:new hash is required")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: "abc"}}}, fmt.Errorf("index:0, field:references.newHash, error:new hash is not valid")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: strings.Repeat("x", 40)}}}, fmt.Errorf("index:0, field:references.nonce, error:reference nonce must be greater than zero")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: strings.Repeat("x", 40), Nonce: 1}}}, fmt.Errorf("index:0, field:references.accountNonce, error:account nonce must be greater than zero")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: strings.Repeat("x", 40), Nonce: 1, AccountNonce: 1}}}, fmt.Errorf("index:0, field:references.fee, error:fee must be numeric")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: strings.Repeat("x", 40), Nonce: 1, AccountNonce: 1, Fee: "0a"}}}, fmt.Errorf("index:0, field:references.fee, error:fee must be numeric")},
			{&PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref", OldHash: strings.Repeat("x", 40), NewHash: strings.Repeat("x", 40), Nonce: 1, AccountNonce: 1, Fee: "1", Objects: []string{"abc"}}}}, fmt.Errorf("index:0, field:references.objects.0, error:object hash is not valid")},
		}

		It("should check cases", func() {
			for _, c := range cases {
				_c := c
				if _c[1] != nil {
					Expect(checkPushNoteSyntax(_c[0].(*PushNote))).To(Equal(_c[1]))
				} else {
					Expect(checkPushNoteSyntax(_c[0].(*PushNote))).To(BeNil())
				}
			}
		})
	})

	Describe(".checkPushedReference", func() {
		var mockKeepers *mocks.MockKeepers
		var mockRepo *mocks.MockBareRepo
		var oldHash = fmt.Sprintf("%x", util.Sha1(util.RandBytes(16)))

		BeforeEach(func() {
			mockKeepers = mocks.NewMockKeepers(ctrl)
			mockRepo = mocks.NewMockBareRepo(ctrl)
		})

		When("old hash is non-zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*types.PushedReference{
					{Name: "refs/heads/master", OldHash: oldHash},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{}),
				}
				gpgKey := &types.GPGPubKey{}
				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash is zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*types.PushedReference{
					{Name: "refs/heads/master", OldHash: strings.Repeat("0", 40)},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{}),
				}
				gpgKey := &types.GPGPubKey{}
				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should not return error about unknown reference", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:0, field:references, error:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash of reference is different from the local hash of same reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*types.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				gpgKey := &types.GPGPubKey{}

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("old hash of reference is non-zero and the local equivalent reference is not accessible", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*types.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(nil, plumbing.ErrReferenceNotFound)
				gpgKey := &types.GPGPubKey{}

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and it is different from the hash of the equivalent local reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*types.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				gpgKey := &types.GPGPubKey{}

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("new hash of reference is not included in the object's list", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*types.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: util.RandString(40),
						Objects: []string{},
					},
				}
				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)
				gpgKey := &types.GPGPubKey{}

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' new hash not included in the list of objects"))
			})
		})

		When("pushed reference nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*types.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: newHash,
						Objects: []string{newHash},
						Nonce:   2,
					},
				}

				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				gpgKey := &types.GPGPubKey{}
				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' has nonce '2', expecting '1'"))
			})
		})

		When("pusher account nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*types.PushedReference{
					{
						Name:         refName,
						OldHash:      oldHash,
						NewHash:      newHash,
						Objects:      []string{newHash},
						Nonce:        1,
						AccountNonce: 12,
					},
				}

				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				gpgKey := &types.GPGPubKey{Address: "pusher_address"}
				accountKeeper := mocks.NewMockAccountKeeper(ctrl)
				accountKeeper.EXPECT().GetAccount(gpgKey.Address).Return(&types.Account{
					Nonce: 10,
				})
				mockKeepers.EXPECT().AccountKeeper().Return(accountKeeper)

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, error:reference 'refs/heads/master' has account nonce '12', expecting '11'"))
			})
		})

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*types.PushedReference{
					{
						Name:         refName,
						OldHash:      oldHash,
						NewHash:      newHash,
						Objects:      []string{newHash},
						Nonce:        1,
						AccountNonce: 11,
					},
				}

				repository := &types.Repository{
					References: types.References(map[string]*types.Reference{
						refName: &types.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				gpgKey := &types.GPGPubKey{Address: "pusher_address"}
				accountKeeper := mocks.NewMockAccountKeeper(ctrl)
				accountKeeper.EXPECT().GetAccount(gpgKey.Address).Return(&types.Account{
					Nonce: 10,
				})
				mockKeepers.EXPECT().AccountKeeper().Return(accountKeeper)

				err = checkPushedReference(mockRepo, refs, repository, gpgKey, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkPushNoteConsistency", func() {
		var mockKeepers *mocks.MockKeepers
		BeforeEach(func() {
			mockKeepers = mocks.NewMockKeepers(ctrl)
		})

		When("no repository with matching name exist", func() {
			BeforeEach(func() {
				tx := &PushNote{RepoName: "unknown"}
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(types.BareRepository())
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				err = checkPushNoteConsistency(tx, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoName, error:repository named 'unknown' is unknown"))
			})
		})

		When("pusher public id is unknown", func() {
			BeforeEach(func() {
				tx := &PushNote{RepoName: "repo1", PusherKeyID: "pkID"}
				mockRepoKeeper := mocks.NewMockRepoKeeper(ctrl)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&types.Repository{CreatorAddress: "addr1"})
				mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
				mockGPGKeeper := mocks.NewMockGPGPubKeyKeeper(ctrl)
				mockGPGKeeper.EXPECT().GetGPGPubKey(tx.PusherKeyID).Return(types.BareGPGPubKey())
				mockKeepers.EXPECT().GPGPubKeyKeeper().Return(mockGPGKeeper)
				err = checkPushNoteConsistency(tx, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherKeyId, error:pusher's public key id 'pkID' is unknown"))
			})
		})
	})

	Describe(".fetchAndCheckReferenceObjects", func() {
		When("object does not exist in the dht", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.targetRepo = mockRepo

				mockDHT := mocks.NewMockDHT(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(nil, fmt.Errorf("object not found"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch object 'obj_hash': object not found"))
			})
		})

		When("object exist in the dht but failed to write to repository", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.targetRepo = mockRepo

				mockDHT := mocks.NewMockDHT(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(fmt.Errorf("something bad"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write fetched object 'obj_hash' to disk: something bad"))
			})
		})

		When("object exist in the dht and successfully written to disk", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 7}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.targetRepo = mockRepo

				mockDHT := mocks.NewMockDHT(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("object exist in the dht, successfully written to disk and object size is different from actual size", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 10}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.targetRepo = mockRepo

				mockDHT := mocks.NewMockDHT(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:size, error:invalid size (10 bytes). actual object size (7 bytes) is different"))
			})
		})
	})
})
