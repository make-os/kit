package crypto_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	. "github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
)

var pk = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBF3D1yoBCACjdSC/KibksNrQ+gMb3Cw0I603SMwK8rvw5rE/L3oif7xc9Ghw
ZeQbSgpNCFVY9yUGX0WznQirAd5o4pleb6p/AmFtj3huLuPQ9IPA5xvPvf8k39Ky
aos5KHLK/tt6f+kG36IQpV2xryZs7ny4tNFKIHcl0HPC1oySFmAo0nVzDcpjFkYU
k2tryQo8JerFfOLp6NwTdXSsqFozKSSXHOwDDi8v811Wik48RKWaJ68LCS50CGFl
NYlYVkmZd29QIqJc4nUXrR/PmZqOklXC3feEJhSlmoFgMAWpfE6ffkGzqK7BQfAh
BarTbNGyV7mGZvY7w1wklFc6dlBGMWrsFZ6JABEBAAG0EEtlbm5lZHkgKHRlc3Qg
MymJAU4EEwEIADgWIQRpC08nO1qMBK/UHh3hTuV6RZk83wUCXcPXKgIbAwULCQgH
AgYVCgkICwIEFgIDAQIeAQIXgAAKCRDhTuV6RZk832u8B/9gZ4cT5rCkUUxH4s6F
oRtnEL01Q+iK9IyissVY1ZMM7p4+u5eXwljCqG5pw/KoHHIOZ98NuytRcgAM9dsi
vaWjKGxEOWD1VeKNEPDHu7KEQBfwYzfz+obf01e89E1NwvTQWmu/lK75hNajZPrh
EBIFoYI8ZiSsCnHESqI8hblezGYhxwXysD6zz3+tE5mcCswT5s95JQ6uYmeWrmlh
1B07BQ7d5GH5XAI+Bg4O90AXODCr4OKnuDcquqkpgwjBs1dDMFOtqn7V3qIsfsQF
cDwi7Nac0GbnW4arjTozjzYwEN34vDxJvvRQNM8467fZh4YHMWVnI80wf/HeI5ZR
ELi6uQENBF3D1yoBCADNLl6k97YZyKO30UE4/tyG0eQuEvCWa504MBIaVNa77F7e
snZaekKFIzrTAZJACu/2uCEJIfNyvsMp8EovVScw3Zm8SK4BVscot1KAntXZlf/3
4vWUnQqUb5ANav3I0l1a5ndtOmQCTuiZ5kW+6eUjra01pt1J9GxUMc/2DDC+HkYY
/emc/Uc44HPbIy8NlGCjSXCG0/QvyB+nHBxQtEAyX/aK5ylUQ/frPakS23yFviZs
cYb3ywAfMadWtchk7eG2ywLHpSVhuKhbHQdTtUSjLhllcjzrfMF1qUplrk+IDnp4
SRwSdbZ2E2CbeL0h/hifzGkYblWdYDe+lh5i+IDvABEBAAGJATYEGAEIACAWIQRp
C08nO1qMBK/UHh3hTuV6RZk83wUCXcPXKgIbDAAKCRDhTuV6RZk832c+CACIpykT
D3ZtAg+YsF2cb0xeQtvK4Hm0q2eaj0ri04b56K8+LeQxruuiQVEffE72lX+Sqpin
765wmOoK26eQ1IlRlwUEgoSusdko2cpgNaC5IgYXyG3pyRQ9wewudXM68jYXy5x9
FmSjybTOkWVO5qudYk2Cu6g4T7UyPrgGJ2iMunjDAVyK+BvhwZhx/HxLBTAx3uve
QpQXS1MnYXkyQz5mbqElHf0ELDX5zQ0JPNEL7CEf9dgBGUo02aGFCl0/oFR7O2el
yYXxF8MfL+q9HPVL7IrFOI3bLtrVuEt1qE6/vCzC804ODi4gfc9a2di3bKpMyoUU
svCU0gx1j1vi1SKS
=vHUA
-----END PGP PUBLIC KEY BLOCK-----`

var _ = Describe("Gpg", func() {

	var err error
	var cfg *config.AppConfig
	var gpgHome string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		gpgHome = cfg.DataDir()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".PGPEntityFromPubKey", func() {
		When("pub key is not valid", func() {
			It("should return err", func() {
				pubKey := "abc"
				_, err := PGPEntityFromPubKey(pubKey)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("openpgp: invalid argument: no armored data found"))
			})
		})

		When("pub key is valid", func() {
			It("should return nil", func() {
				_, err := PGPEntityFromPubKey(pk)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".GetGPGPublicKey", func() {
		When("program execution fail", func() {
			It("should return error", func() {
				_, err := GetGPGPublicKey("unknown_key", "unknown_program", "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(`failed to get public key (target id: unknown_key): ` +
					`exec: "unknown_program": executable file not found in $PATH`))
			})
		})

		When("key doesn't exist", func() {
			It("should return err", func() {
				_, err := GetGPGPublicKey("unknown_key", testutil.GPGProgramPath, "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("gpg public key not found"))
			})
		})

		When("key exist", func() {
			var keyID string

			BeforeEach(func() {
				keyID = testutil.CreateGPGKey(testutil.GPGProgramPath, gpgHome)
			})

			It("should return nil", func() {
				en, err := GetGPGPublicKey(keyID, testutil.GPGProgramPath, gpgHome)
				Expect(err).To(BeNil())
				Expect(en.PrimaryKey.KeyIdString()).To(Equal(keyID))
			})
		})
	})

	Describe(".GetGPGPublicKeyStr", func() {
		When("program execution fail", func() {
			It("should return error", func() {
				_, err := GetGPGPublicKeyStr("unknown_key", "unknown_program", "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(`failed to get public key (target id: unknown_key): ` +
					`exec: "unknown_program": executable file not found in $PATH`))
			})
		})

		When("key doesn't exist", func() {
			It("should return err", func() {
				_, err := GetGPGPublicKeyStr("unknown_key", testutil.GPGProgramPath, "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("gpg public key not found"))
			})
		})

		When("key exist", func() {
			var keyID string

			BeforeEach(func() {
				keyID = testutil.CreateGPGKey(testutil.GPGProgramPath, gpgHome)
			})

			It("should return nil", func() {
				pkStr, err := GetGPGPublicKeyStr(keyID, testutil.GPGProgramPath, gpgHome)
				Expect(err).To(BeNil())
				Expect(pkStr).To(ContainSubstring("BEGIN PGP PUBLIC KEY BLOCK"))
			})
		})
	})

	Describe(".GetGPGPrivateKey", func() {
		When("program execution fail", func() {
			It("should return error", func() {
				_, err := GetGPGPrivateKey("unknown_key", "unknown_program", "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(`failed to get private key (target id: unknown_key): ` +
					`exec: "unknown_program": executable file not found in $PATH`))
			})
		})

		When("key doesn't exist", func() {
			It("should return err", func() {
				_, err := GetGPGPrivateKey("unknown_key", testutil.GPGProgramPath, "")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("gpg private key not found"))
			})
		})

		When("key exist", func() {
			var keyID string

			BeforeEach(func() {
				keyID = testutil.CreateGPGKey(testutil.GPGProgramPath, gpgHome)
			})

			It("should return nil", func() {
				en, err := GetGPGPrivateKey(keyID, testutil.GPGProgramPath, gpgHome)
				Expect(err).To(BeNil())
				Expect(en.PrimaryKey.KeyIdString()).To(Equal(keyID))
			})
		})
	})

	Describe(".VerifyGPGSignature", func() {
		When("signature is valid", func() {
			sig := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEEaQtPJztajASv1B4d4U7lekWZPN8FAl3HsOMACgkQ4U7lekWZ
PN9BmAf/ZoEmug1YK/BZFj8QLAOeegNs/saFUFPVCtkIxdOXWFcAR2HD3E8ztYJ4
AvzHLagSPJFVBgtFt7lzq8pXNURUd2N9oAOWcCu6HJiId4HsXf/8U/oQTipSTduR
U/Dzo6Du8cRybvLMckfc6hd7ZI0yN0AL/zZrqMSHAWhIB3o67d5HiGUcPs7jown7
glD152QFs+mZ6SzagRLIvGQbQvQXJNN1+gC0eVSW3WbKYrkwq0erxnFOONCFbU7L
N9e/8Q1a9wUAXJcs+t5Jme/ZAS6MJAdnbr9fC0C3HyiotcUi0mxAZkeJ8j8EzIcD
yQdUMpy+5xo/JqR/GhRLO3YZSBmw/w==
=nyXY
-----END PGP SIGNATURE-----`

			msg := `tree e2f93d459d5ff5af92f0a089601245adf1a9cbe3
parent b21512fd9b37857244ef526f7a559528471552e5
author Kennedy Idialu <email@example.com> 1573368035 +0100
committer Kennedy Idialu <email@example.com> 1573368035 +0100

Changes made
`
			It("should return true", func() {
				entity, err := PGPEntityFromPubKey(pk)
				Expect(err).To(BeNil())
				ok, err := VerifyGPGSignature(entity, []byte(sig), []byte(msg))
				Expect(err).To(BeNil())
				Expect(ok).To(BeTrue())
			})
		})

		When("signature is invalid", func() {
			sig := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEEaQtPJzt=
=vLWF
-----END PGP SIGNATURE-----`

			msg := `tree acd2ac9a24b158bfbc2848cee76986600716f4d6
parent 1f8e52b76766f2c9e4e188c63150baed3e083e7d
author Kennedy Idialu <kennedyidialu@gmail.com> 1573236412 +0100
committer Kennedy Idialu <kennedyidialu@gmail.com> 1573236412 +0100

lala
`
			It("should return true", func() {
				entity, err := PGPEntityFromPubKey(pk)
				Expect(err).To(BeNil())
				ok, err := VerifyGPGSignature(entity, []byte(sig), []byte(msg))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to read signature body: openpgp: invalid data: armor invalid"))
				Expect(ok).To(BeFalse())
			})
		})
	})
})
