module github.com/makeos/mosdef

go 1.13

replace (
	github.com/bitfield/script => github.com/ncodes/script v0.14.1-0.20191105145315-f4455694bf0d
	github.com/tendermint/tendermint => github.com/ncodes/tendermint v0.32.6
)

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Workiva/go-datastructures v1.0.50
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a
	github.com/bitfield/script v0.14.0
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/c-bata/go-prompt v0.2.3
	github.com/dedis/drand v0.5.0
	github.com/dgraph-io/badger v1.6.0
	github.com/dustin/go-humanize v1.0.0
	github.com/ellcrys/go-ethereum v1.8.7
	github.com/ellcrys/go-prompt v1.2.1
	github.com/fastly/go-utils v0.0.0-20180712184237-d95a45783239 // indirect
	github.com/fatih/color v1.7.0
	github.com/fatih/structs v1.1.0
	github.com/go-ozzo/ozzo-validation v3.6.0+incompatible
	github.com/gobuffalo/packr v1.30.1
	github.com/gogo/protobuf v1.3.0
	github.com/golang/mock v1.3.1
	github.com/google/martian v2.1.0+incompatible
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/rpc v1.2.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/hokaccha/go-prettyjson v0.0.0-20190818114111-108c894c2c0e // indirect
	github.com/howeyc/gopass v0.0.0-20190910152052-7cb4b85ec19c // indirect
	github.com/imdario/mergo v0.3.8
	github.com/imroc/req v0.2.4
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869 // indirect
	github.com/k0kubun/colorstring v0.0.0-20150214042306-9440f1994b88 // indirect
	github.com/k0kubun/pp v3.0.1+incompatible
	github.com/lestrrat-go/file-rotatelogs v2.2.0+incompatible
	github.com/lestrrat-go/strftime v0.0.0-20190725011945-5c849dd2c51d // indirect
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-peer v0.2.0
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mattn/go-tty v0.0.0-20190424173100-523744f04859 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/motemen/go-quickfix v0.0.0-20160413151302-5c522febc679 // indirect
	github.com/motemen/gore v0.4.1 // indirect
	github.com/ncodes/go-prettyjson v0.0.0-20180528130907-d229c224a219
	github.com/olebedev/emitter v0.0.0-20190110104742-e8d1457e6aee
	github.com/onsi/ginkgo v1.7.0
	github.com/onsi/gomega v1.4.3
	github.com/peterh/liner v1.1.0 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a // indirect
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/robertkrimen/otto v0.0.0-20180617131154-15f95af6e78d
	github.com/shopspring/decimal v0.0.0-20190905144223-a36b5d85f337
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/objx v0.2.0
	github.com/stumble/gorocksdb v0.0.3 // indirect
	github.com/tebeka/strftime v0.1.3 // indirect
	github.com/tendermint/iavl v0.12.4
	github.com/tendermint/tendermint v0.32.6
	github.com/tendermint/tm-db v0.2.0
	github.com/thoas/go-funk v0.4.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.dedis.ch/kyber/v3 v3.0.3-0.20190501101437-0324e4ea86f1
	golang.org/x/crypto v0.0.0-20191029031824-8986dd9e96cf
	golang.org/x/net v0.0.0-20191101175033-0deb6923b6d9 // indirect
	golang.org/x/sys v0.0.0-20191029155521-f43be2a4598c // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/oleiade/lane.v1 v1.0.0
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1
)
