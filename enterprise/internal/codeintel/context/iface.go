package context

import (
	"context"

	codenavtypes "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
)

type CodeNavService interface {
	GetClosestDumpsForBlob(ctx context.Context, repositoryID int, commit, path string, exactPath bool, indexer string) (_ []uploadsshared.Dump, err error)
	GetFullSCIPNameByDescriptor(ctx context.Context, uploadID []int, symbolNames []string) (names []*types.SCIPNames, err error)
	GetUploadLocations(ctx context.Context, args codenavtypes.RequestArgs, requestState codenavtypes.RequestState, locations []shared.Location, includeFallbackLocations bool) ([]shared.UploadLocation, error)
	GetLocationByExplodedSymbol(ctx context.Context, symbolName string, uploadID int, scipFieldName string) (locations []shared.Location, err error)
}
