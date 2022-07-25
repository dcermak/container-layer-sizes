module github.com/dcermak/container-layer-sizes

go 1.16

require (
	github.com/containers/image/v5 v5.22.0
	github.com/containers/storage v1.42.0
	github.com/docker/distribution v2.8.1+incompatible
	github.com/google/uuid v1.3.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/mholt/archiver/v4 v4.0.0-alpha.7
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/opencontainers/umoci v0.4.7
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli/v2 v2.8.1
	go.opentelemetry.io/otel v1.3.0 // indirect
)

exclude github.com/docker/distributions v2.8.0+incompatible
