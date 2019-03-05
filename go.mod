module github.com/jfrog/jfrog-cli-go

require (
	github.com/buger/jsonparser v0.0.0-20180910192245-6acdf747ae99
	github.com/codegangsta/cli v1.20.0
	github.com/jfrog/gocmd v0.1.7
	github.com/jfrog/gofrog v1.0.4
	github.com/jfrog/jfrog-client-go v0.3.2
	github.com/magiconair/properties v1.8.0
	github.com/mattn/go-shellwords v1.0.3
	github.com/mholt/archiver v2.1.0+incompatible
	github.com/spf13/viper v1.2.1
	golang.org/x/crypto v0.0.0-20181203042331-505ab145d0a9
	gopkg.in/src-d/go-git-fixtures.v3 v3.3.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/jfrog/jfrog-client-go => github.com/elioengcomp/jfrog-client-go v0.3.1-0.20190305181546-998d8e42b9e6

replace github.com/jfrog/gocmd => github.com/elioengcomp/gocmd v0.1.6-0.20190305184023-216dc314212c
