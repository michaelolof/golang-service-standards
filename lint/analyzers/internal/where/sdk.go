package where

import (
	"go/types"
	"strings"
)

// SDK is one SDK the standard keeps at the edge (LAY-7, ABS-7).
type SDK struct {
	// Path is the import path prefix: a module, or a package inside one.
	Path string
	// Homes are the adapter folders, under internal/adapters, that may import
	// it. An SDK used by several systems, such as the AWS SDK's shared packages,
	// has none, and depguard alone keeps it in adapters.
	Homes []string
	// Driver marks a database driver. Repositories import it next to the store
	// adapter that opens it (LAY-8), and that adapter may hand its types to them.
	Driver bool
}

// SDKs are the SDKs depguard keeps in adapters, repositories, cmd and pkg, with
// the adapter each belongs in. ABS-7 names an adapter for its external system,
// and LAY-8 adds "store" to a database or object store holding the service's
// data. depguard's sdk-confinement rule in lint/.golangci.yml lists the same
// modules.
var SDKs = []SDK{
	{Path: "github.com/jackc/pgx", Homes: []string{"crdbstore", "pgstore"}, Driver: true},
	{Path: "go.mongodb.org/mongo-driver", Homes: []string{"mongostore"}, Driver: true},
	{Path: "github.com/gocql/gocql", Homes: []string{"scyllastore", "cassandrastore"}, Driver: true},
	{Path: "github.com/apache/cassandra-gocql-driver", Homes: []string{"scyllastore", "cassandrastore"}, Driver: true},
	{Path: "github.com/redis/go-redis", Homes: []string{"redis"}},
	{Path: "github.com/hibiken/asynq", Homes: []string{"redis"}},
	{Path: "github.com/twmb/franz-go", Homes: []string{"kafka"}},
	{Path: "github.com/minio/minio-go", Homes: []string{"s3store"}},
	{Path: "github.com/aws/aws-sdk-go-v2/service/s3", Homes: []string{"s3store"}},
	{Path: "github.com/aws/aws-sdk-go-v2/feature/s3", Homes: []string{"s3store"}},
	{Path: "github.com/aws/aws-sdk-go-v2/service/kms", Homes: []string{"awskms"}},
	{Path: "github.com/aws/aws-sdk-go-v2"},
	{Path: "github.com/aws/smithy-go"},
}

// SDKOf returns the most specific SDK that path belongs to, and false when it's
// none of them.
func SDKOf(path string) (SDK, bool) {
	var (
		found SDK
		ok    bool
	)

	for _, sdk := range SDKs {
		if (path == sdk.Path || strings.HasPrefix(path, sdk.Path+"/")) && len(sdk.Path) > len(found.Path) {
			found, ok = sdk, true
		}
	}

	return found, ok
}

// SDKIn returns the package path of the first SDK type t mentions, or "". With
// drivers, database drivers' types don't count: a store adapter and its
// repositories share them.
func SDKIn(t types.Type, drivers bool) string {
	found := ""

	Walk(t, func(named *types.TypeName) bool {
		if named.Pkg() == nil {
			return false
		}

		sdk, ok := SDKOf(named.Pkg().Path())
		if !ok || (drivers && sdk.Driver) {
			return false
		}

		found = named.Pkg().Path()

		return true
	})

	return found
}
