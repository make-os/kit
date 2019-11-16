package repo

import (
	"crypto/rsa"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.EngineConfig
	var repo *Repo
	var path string
	var gpgKeyID string
	var pubKey string

	var gpgPubKeyGetter = func(pkId string) (string, error) {
		return pubKey, nil
	}

	var gpgPubKeyGetterWithErr = func(err error) func(pkId string) (string, error) {
		return func(pkId string) (string, error) {
			return "", err
		}
	}

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		gpgKeyID = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey, err = crypto.GetGPGPublicKeyStr(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
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
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".checkCommit", func() {
		var cob *object.Commit
		var err error

		When("txline is not set", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				err = checkCommit(cob, false, repo, gpgPubKeyGetter)
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
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				err = checkCommit(cob, false, repo, gpgPubKeyGetter)
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
				err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. Please sign the commit with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. Please sign the commit with your gpg key"))
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
				err = checkCommit(cob, false, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..Public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*Public key.*was not found"))
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
				err = checkCommit(cob, false, repo, gpgPubKeyGetter)
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
				err = checkCommit(cob, false, repo, gpgPubKeyGetter)
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. Please sign the tag with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. Please sign the tag with your gpg key"))
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..Public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*Public key.*was not found"))
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
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
				err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
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
				change := &ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				err = validateChange(repo, change, gpgPubKeyGetter)
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

				change := &ItemChange{Item: &Obj{Name: "refs/heads/master", Data: cob.Hash.String()}}
				err = validateChange(repo, change, gpgPubKeyGetter)
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

				change := &ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tob.Hash.String()}}
				err = validateChange(repo, change, gpgPubKeyGetter)
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

				change := &ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tagRef.Target().String()}}
				err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
