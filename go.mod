module github.com/dcermak/container-layer-sizes

go 1.16

require (
	github.com/containers/image/v5 v5.21.0
	github.com/containers/storage v1.39.0
	github.com/docker/distribution v2.8.1+incompatible
	github.com/google/uuid v1.3.0
	github.com/mattn/go-sqlite3 v1.14.11
	github.com/mholt/archiver/v4 v4.0.0-alpha.5
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli/v2 v2.3.0
)

exclude github.com/docker/distributions v2.8.0+incompatible
