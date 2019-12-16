module github.com/jfrog/jfrog-cli-go

require (
	9fans.net/go v0.0.2 // indirect
	github.com/alecthomas/gometalinter v3.0.0+incompatible // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/buger/jsonparser v0.0.0-20180910192245-6acdf747ae99
	github.com/codegangsta/cli v1.20.0
	github.com/fatih/gomodifytags v1.0.1 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/jfrog/gocmd v0.1.7
	github.com/jfrog/gofrog v1.0.4
	github.com/jfrog/jfrog-client-go v0.3.2
	github.com/magiconair/properties v1.8.0
	github.com/mattn/go-shellwords v1.0.3
	github.com/mdempsky/gocode v0.0.0-20191202075140-939b4a677f2f // indirect
	github.com/mholt/archiver v2.1.0+incompatible
	github.com/rogpeppe/godef v1.1.1 // indirect
	github.com/spf13/viper v1.2.1
	github.com/sqs/goreturns v0.0.0-20181028201513-538ac6014518 // indirect
	github.com/tpng/gopkgs v0.0.0-20180428091733-81e90e22e204 // indirect
	github.com/zmb3/goaddimport v0.0.0-20170810013102-4ab94a07ab86 // indirect
	github.com/zmb3/gogetdoc v0.0.0-20190228002656-b37376c5da6a // indirect
	golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449 // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/tools v0.0.0-20191216173652-a0e659d51361 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20191105091915-95d230a53780 // indirect
	gopkg.in/src-d/go-git-fixtures.v3 v3.3.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/jfrog/jfrog-client-go => github.com/elioengcomp/jfrog-client-go v0.3.1-0.20190305181546-998d8e42b9e6

replace github.com/jfrog/gocmd => github.com/elioengcomp/gocmd v0.1.6-0.20191214005640-a12473fdd258

replace github.com/jfrog/gofrog => github.com/elioengcomp/gofrog v1.0.5-0.20190320220736-a9dafc930911

go 1.13
