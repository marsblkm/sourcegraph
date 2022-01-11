package resolvers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/shared"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/bloomfilter"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func TestImplementations(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	mockPositionAdjuster := noopPositionAdjuster()

	// Empty result set (prevents nil pointer as scanner is always non-nil)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(dbstore.PackageReferenceScannerFromSlice(), 0, nil)

	locations := []lsifstore.Location{
		{DumpID: 51, Path: "a.go", Range: testRange1},
		{DumpID: 51, Path: "b.go", Range: testRange2},
		{DumpID: 51, Path: "a.go", Range: testRange3},
		{DumpID: 51, Path: "b.go", Range: testRange4},
		{DumpID: 51, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[:1], 1, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[1:4], 3, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[4:], 1, nil)

	uploads := []dbstore.Dump{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	resolver := newQueryResolver(
		mockDBStore,
		mockLSIFStore,
		newCachedCommitChecker(mockGitserverClient),
		mockPositionAdjuster,
		42,
		"deadbeef",
		"s1/main.go",
		uploads,
		newOperations(&observation.TestContext),
		authz.NewMockSubRepoPermissionChecker(),
	)
	adjustedLocations, _, err := resolver.Implementations(context.Background(), 10, 20, 50, "")
	if err != nil {
		t.Fatalf("unexpected error querying implementations: %s", err)
	}

	expectedLocations := []AdjustedLocation{
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[1], Path: "sub2/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange2},
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
		{Dump: uploads[1], Path: "sub2/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange4},
		{Dump: uploads[1], Path: "sub2/c.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange5},
	}
	if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}
}

func TestImplementationsWithSubRepoPermissions(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	mockPositionAdjuster := noopPositionAdjuster()

	// Empty result set (prevents nil pointer as scanner is always non-nil)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(dbstore.PackageReferenceScannerFromSlice(), 0, nil)

	locations := []lsifstore.Location{
		{DumpID: 51, Path: "a.go", Range: testRange1},
		{DumpID: 51, Path: "b.go", Range: testRange2},
		{DumpID: 51, Path: "a.go", Range: testRange3},
		{DumpID: 51, Path: "b.go", Range: testRange4},
		{DumpID: 51, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[:1], 1, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[1:4], 3, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[4:], 1, nil)

	uploads := []dbstore.Dump{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}

	// Applying sub-repo permissions
	checker := authz.NewMockSubRepoPermissionChecker()

	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})

	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if content.Path == "sub2/a.go" {
			return authz.Read, nil
		}
		return authz.None, nil
	})

	resolver := newQueryResolver(
		mockDBStore,
		mockLSIFStore,
		newCachedCommitChecker(mockGitserverClient),
		mockPositionAdjuster,
		42,
		"deadbeef",
		"s1/main.go",
		uploads,
		newOperations(&observation.TestContext),
		checker,
	)

	ctx := context.Background()
	adjustedLocations, _, err := resolver.Implementations(actor.WithActor(ctx, &actor.Actor{UID: 1}), 10, 20, 50, "")
	if err != nil {
		t.Fatalf("unexpected error querying implementations: %s", err)
	}

	expectedLocations := []AdjustedLocation{
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
	}
	if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}
}

func TestImplementationsRemote(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	mockPositionAdjuster := noopPositionAdjuster()

	definitionUploads := []dbstore.Dump{
		{ID: 150, Commit: "deadbeef1", Root: "sub1/"},
		{ID: 151, Commit: "deadbeef2", Root: "sub2/"},
		{ID: 152, Commit: "deadbeef3", Root: "sub3/"},
		{ID: 153, Commit: "deadbeef4", Root: "sub4/"},
	}
	mockDBStore.DefinitionDumpsFunc.PushReturn(definitionUploads, nil)

	referenceUploads := []dbstore.Dump{
		{ID: 250, Commit: "deadbeef1", Root: "sub1/"},
		{ID: 251, Commit: "deadbeef2", Root: "sub2/"},
		{ID: 252, Commit: "deadbeef3", Root: "sub3/"},
		{ID: 253, Commit: "deadbeef4", Root: "sub4/"},
	}
	mockDBStore.GetDumpsByIDsFunc.PushReturn(referenceUploads[:2], nil)
	mockDBStore.GetDumpsByIDsFunc.PushReturn(referenceUploads[2:], nil)

	filter, err := bloomfilter.CreateFilter([]string{"padLeft", "pad_left", "pad-left", "left_pad"})
	if err != nil {
		t.Fatalf("unexpected error encoding bloom filter: %s", err)
	}
	scanner1 := dbstore.PackageReferenceScannerFromSlice(
		shared.PackageReference{Package: shared.Package{DumpID: 250}, Filter: filter},
		shared.PackageReference{Package: shared.Package{DumpID: 251}, Filter: filter},
	)
	scanner2 := dbstore.PackageReferenceScannerFromSlice(
		shared.PackageReference{Package: shared.Package{DumpID: 252}, Filter: filter},
		shared.PackageReference{Package: shared.Package{DumpID: 253}, Filter: filter},
	)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(scanner1, 4, nil)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(scanner2, 2, nil)

	// upload #150/#250's commits no longer exists; all others do
	mockGitserverClient.CommitExistsFunc.PushReturn(false, nil) // #150
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #151
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #152
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #153
	mockGitserverClient.CommitExistsFunc.PushReturn(false, nil) // #250
	mockGitserverClient.CommitExistsFunc.SetDefaultReturn(true, nil)

	monikers := []precise.MonikerData{
		{Kind: "implementation", Scheme: "tsc", Identifier: "padLeft", PackageInformationID: "51"},
		{Kind: "export", Scheme: "tsc", Identifier: "pad_left", PackageInformationID: "52"},
	}
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[0]}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[1]}}, nil)

	packageInformation1 := precise.PackageInformationData{Name: "leftpad", Version: "0.1.0"}
	packageInformation2 := precise.PackageInformationData{Name: "leftpad", Version: "0.2.0"}
	mockLSIFStore.PackageInformationFunc.PushReturn(packageInformation1, true, nil)
	mockLSIFStore.PackageInformationFunc.PushReturn(packageInformation2, true, nil)

	locations := []lsifstore.Location{
		{DumpID: 51, Path: "a.go", Range: testRange1},
		{DumpID: 51, Path: "b.go", Range: testRange2},
		{DumpID: 51, Path: "a.go", Range: testRange3},
		{DumpID: 51, Path: "b.go", Range: testRange4},
		{DumpID: 51, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[:1], 1, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[1:4], 3, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[4:5], 1, nil)

	monikerLocations := []lsifstore.Location{
		{DumpID: 53, Path: "a.go", Range: testRange1},
		{DumpID: 53, Path: "b.go", Range: testRange2},
		{DumpID: 53, Path: "a.go", Range: testRange3},
		{DumpID: 53, Path: "b.go", Range: testRange4},
		{DumpID: 53, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[0:1], 1, nil) // defs
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[1:2], 1, nil) // impls batch 1
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[2:], 3, nil)  // impls batch 2

	uploads := []dbstore.Dump{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	resolver := newQueryResolver(
		mockDBStore,
		mockLSIFStore,
		newCachedCommitChecker(mockGitserverClient),
		mockPositionAdjuster,
		42,
		"deadbeef",
		"s1/main.go",
		uploads,
		newOperations(&observation.TestContext),
		authz.NewMockSubRepoPermissionChecker(),
	)
	adjustedLocations, _, err := resolver.Implementations(context.Background(), 10, 20, 50, "")
	if err != nil {
		t.Fatalf("unexpected error querying references: %s", err)
	}

	expectedLocations := []AdjustedLocation{
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[1], Path: "sub2/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange2},
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
		{Dump: uploads[1], Path: "sub2/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange4},
		{Dump: uploads[1], Path: "sub2/c.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange5},
		{Dump: uploads[3], Path: "sub4/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[3], Path: "sub4/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange2},
		{Dump: uploads[3], Path: "sub4/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
		{Dump: uploads[3], Path: "sub4/b.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange4},
		{Dump: uploads[3], Path: "sub4/c.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange5},
	}
	if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}

	if history := mockDBStore.DefinitionDumpsFunc.History(); len(history) != 1 {
		t.Fatalf("unexpected call count for dbstore.DefinitionDump. want=%d have=%d", 1, len(history))
	} else {
		expectedMonikers := []precise.QualifiedMonikerData{
			{MonikerData: monikers[0], PackageInformationData: packageInformation1},
		}
		if diff := cmp.Diff(expectedMonikers, history[0].Arg1); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}
	}

	if history := mockLSIFStore.BulkMonikerResultsFunc.History(); len(history) != 3 {
		t.Fatalf("unexpected call count for lsifstore.BulkMonikerResults. want=%d have=%d", 3, len(history))
	} else {
		if diff := cmp.Diff([]int{151, 152, 153}, history[0].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]precise.MonikerData{monikers[0]}, history[0].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]int{251}, history[1].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]precise.MonikerData{monikers[1]}, history[1].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]int{252, 253}, history[2].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]precise.MonikerData{monikers[1]}, history[2].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}
	}
}

func TestImplementationsRemoteWithSubRepoPermissions(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	mockPositionAdjuster := noopPositionAdjuster()

	definitionUploads := []dbstore.Dump{
		{ID: 150, Commit: "deadbeef1", Root: "sub1/"},
		{ID: 151, Commit: "deadbeef2", Root: "sub2/"},
		{ID: 152, Commit: "deadbeef3", Root: "sub3/"},
		{ID: 153, Commit: "deadbeef4", Root: "sub4/"},
	}
	mockDBStore.DefinitionDumpsFunc.PushReturn(definitionUploads, nil)

	referenceUploads := []dbstore.Dump{
		{ID: 250, Commit: "deadbeef1", Root: "sub1/"},
		{ID: 251, Commit: "deadbeef2", Root: "sub2/"},
		{ID: 252, Commit: "deadbeef3", Root: "sub3/"},
		{ID: 253, Commit: "deadbeef4", Root: "sub4/"},
	}
	mockDBStore.GetDumpsByIDsFunc.PushReturn(referenceUploads[:2], nil)
	mockDBStore.GetDumpsByIDsFunc.PushReturn(referenceUploads[2:], nil)

	filter, err := bloomfilter.CreateFilter([]string{"padLeft", "pad_left", "pad-left", "left_pad"})
	if err != nil {
		t.Fatalf("unexpected error encoding bloom filter: %s", err)
	}
	scanner1 := dbstore.PackageReferenceScannerFromSlice(
		shared.PackageReference{Package: shared.Package{DumpID: 250}, Filter: filter},
		shared.PackageReference{Package: shared.Package{DumpID: 251}, Filter: filter},
	)
	scanner2 := dbstore.PackageReferenceScannerFromSlice(
		shared.PackageReference{Package: shared.Package{DumpID: 252}, Filter: filter},
		shared.PackageReference{Package: shared.Package{DumpID: 253}, Filter: filter},
	)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(scanner1, 4, nil)
	mockDBStore.ReferenceIDsAndFiltersFunc.PushReturn(scanner2, 2, nil)

	// upload #150/#250's commits no longer exists; all others do
	mockGitserverClient.CommitExistsFunc.PushReturn(false, nil) // #150
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #151
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #152
	mockGitserverClient.CommitExistsFunc.PushReturn(true, nil)  // #153
	mockGitserverClient.CommitExistsFunc.PushReturn(false, nil) // #250
	mockGitserverClient.CommitExistsFunc.SetDefaultReturn(true, nil)

	monikers := []precise.MonikerData{
		{Kind: "implementation", Scheme: "tsc", Identifier: "padLeft", PackageInformationID: "51"},
		{Kind: "export", Scheme: "tsc", Identifier: "pad_left", PackageInformationID: "52"},
	}
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[0]}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{}}, nil)
	mockLSIFStore.MonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[1]}}, nil)

	packageInformation1 := precise.PackageInformationData{Name: "leftpad", Version: "0.1.0"}
	packageInformation2 := precise.PackageInformationData{Name: "leftpad", Version: "0.2.0"}
	mockLSIFStore.PackageInformationFunc.PushReturn(packageInformation1, true, nil)
	mockLSIFStore.PackageInformationFunc.PushReturn(packageInformation2, true, nil)

	locations := []lsifstore.Location{
		{DumpID: 51, Path: "a.go", Range: testRange1},
		{DumpID: 51, Path: "b.go", Range: testRange2},
		{DumpID: 51, Path: "a.go", Range: testRange3},
		{DumpID: 51, Path: "b.go", Range: testRange4},
		{DumpID: 51, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[:1], 1, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[1:4], 3, nil)
	mockLSIFStore.ImplementationsFunc.PushReturn(locations[4:5], 1, nil)

	monikerLocations := []lsifstore.Location{
		{DumpID: 53, Path: "a.go", Range: testRange1},
		{DumpID: 53, Path: "b.go", Range: testRange2},
		{DumpID: 53, Path: "a.go", Range: testRange3},
		{DumpID: 53, Path: "b.go", Range: testRange4},
		{DumpID: 53, Path: "c.go", Range: testRange5},
	}
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[0:1], 1, nil) // defs
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[1:2], 1, nil) // impls batch 1
	mockLSIFStore.BulkMonikerResultsFunc.PushReturn(monikerLocations[2:], 3, nil)  // impls batch 2

	uploads := []dbstore.Dump{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}

	// Applying sub-repo permissions
	checker := authz.NewMockSubRepoPermissionChecker()

	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})

	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if content.Path == "sub2/a.go" || content.Path == "sub4/a.go" {
			return authz.Read, nil
		}
		return authz.None, nil
	})

	resolver := newQueryResolver(
		mockDBStore,
		mockLSIFStore,
		newCachedCommitChecker(mockGitserverClient),
		mockPositionAdjuster,
		42,
		"deadbeef",
		"s1/main.go",
		uploads,
		newOperations(&observation.TestContext),
		checker,
	)

	ctx := context.Background()
	adjustedLocations, _, err := resolver.Implementations(actor.WithActor(ctx, &actor.Actor{UID: 1}), 10, 20, 50, "")
	if err != nil {
		t.Fatalf("unexpected error querying references: %s", err)
	}

	expectedLocations := []AdjustedLocation{
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[1], Path: "sub2/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
		{Dump: uploads[3], Path: "sub4/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange1},
		{Dump: uploads[3], Path: "sub4/a.go", AdjustedCommit: "deadbeef", AdjustedRange: testRange3},
	}
	if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}

	if history := mockDBStore.DefinitionDumpsFunc.History(); len(history) != 1 {
		t.Fatalf("unexpected call count for dbstore.DefinitionDump. want=%d have=%d", 1, len(history))
	} else {
		expectedMonikers := []precise.QualifiedMonikerData{
			{MonikerData: monikers[0], PackageInformationData: packageInformation1},
		}
		if diff := cmp.Diff(expectedMonikers, history[0].Arg1); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}
	}

	if history := mockLSIFStore.BulkMonikerResultsFunc.History(); len(history) != 3 {
		t.Fatalf("unexpected call count for lsifstore.BulkMonikerResults. want=%d have=%d", 3, len(history))
	} else {
		if diff := cmp.Diff([]int{151, 152, 153}, history[0].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]precise.MonikerData{monikers[0]}, history[0].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]int{251}, history[1].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]precise.MonikerData{monikers[1]}, history[1].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]int{252, 253}, history[2].Arg2); diff != "" {
			t.Errorf("unexpected ids (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]precise.MonikerData{monikers[1]}, history[2].Arg3); diff != "" {
			t.Errorf("unexpected monikers (-want +got):\n%s", diff)
		}
	}
}
