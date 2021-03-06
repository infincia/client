// Copyright 2017 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package git

import (
	"context"
	"testing"

	"github.com/keybase/client/go/externals"
	"github.com/keybase/client/go/kbfs"
	"github.com/keybase/client/go/kbtest"
	"github.com/keybase/client/go/libkb"
	"github.com/keybase/client/go/protocol/keybase1"
	"github.com/keybase/client/go/teams"
	"github.com/stretchr/testify/require"
)

// Copied from the teams tests.
func SetupTest(tb testing.TB, name string, depth int) (tc libkb.TestContext) {
	tc = libkb.SetupTest(tb, name, depth+1)
	tc.G.SetServices(externals.GetServices())
	teams.NewTeamLoaderAndInstall(tc.G)
	return tc
}

func doPut(t *testing.T, g *libkb.GlobalContext, teamName string, repoID string, repoName string) {
	err := PutMetadata(context.TODO(), g, keybase1.PutGitMetadataArg{
		Folder: keybase1.Folder{
			Name:       teamName,
			Private:    true,
			FolderType: keybase1.FolderType_TEAM,
		},
		RepoID: keybase1.RepoID(repoID),
		Metadata: keybase1.GitLocalMetadata{
			RepoName: keybase1.GitRepoName(repoName),
		},
	})
	require.NoError(t, err)
}

func TestPutAndGet(t *testing.T) {
	tc := SetupTest(t, "team", 1)
	defer tc.Cleanup()

	// Note that the length limit for a team name, with the additional suffix
	// below, is 16 characters. We have 5 to play with, including the implicit
	// underscore after the prefix.
	u, err := kbtest.CreateAndSignupFakeUser("t", tc.G)
	require.NoError(t, err)

	// Create two teams, so that we can test filtering by TeamID.
	teamName1 := u.Username + "t1"
	err = teams.CreateRootTeam(context.TODO(), tc.G, teamName1)
	require.NoError(t, err)
	team1, err := tc.G.GetTeamLoader().Load(context.Background(), keybase1.LoadTeamArg{Name: teamName1})
	require.NoError(t, err)

	teamName2 := u.Username + "t2"
	err = teams.CreateRootTeam(context.TODO(), tc.G, teamName2)
	require.NoError(t, err)

	// Create two git repos, one in each team. Remember that all we're
	// "creating" here is metadata.
	doPut(t, tc.G, teamName1, "abc123", "repoNameFirst")
	doPut(t, tc.G, teamName2, "def456", "repoNameSecond")
	expectedIDNames := map[string]string{
		"abc123": "repoNameFirst",
		"def456": "repoNameSecond",
	}

	// Get all repos, and make sure both come back.
	allRepos, err := GetAllMetadata(context.Background(), tc.G)
	require.NoError(t, err)
	require.Equal(t, 2, len(allRepos), "expected to get both repos back, found: %d", len(allRepos))
	for _, repo := range allRepos {
		require.Equal(t, expectedIDNames[string(repo.RepoID)], string(repo.LocalMetadata.RepoName))
		require.Equal(t, repo.Folder.FolderType, keybase1.FolderType_TEAM)
		require.Equal(t, repo.Folder.Private, true)
		require.Equal(t, repo.ServerMetadata.LastModifyingUsername, u.Username)
	}

	// Now get the repos for just one team. Should be only one of the two we just created.
	oneRepo, err := GetMetadata(context.Background(), tc.G, keybase1.Folder{
		Name:       teamName1,
		Private:    true,
		FolderType: keybase1.FolderType_TEAM,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(oneRepo), "expected to get only one repo back, found: %d", len(oneRepo))
	require.Equal(t, "repoNameFirst", string(oneRepo[0].LocalMetadata.RepoName))
	require.Equal(t, kbtest.DefaultDeviceName, oneRepo[0].ServerMetadata.LastModifyingDeviceName)
	require.Equal(t, string(team1.Chain.Id+"_abc123"), oneRepo[0].GlobalUniqueID)
	require.Equal(t, "keybase://team/"+teamName1+"/repoNameFirst", oneRepo[0].RepoUrl)
}

func TestPutAndGetImplicitTeam(t *testing.T) {
	tc := SetupTest(t, "team", 1)
	defer tc.Cleanup()

	u1, err := kbtest.CreateAndSignupFakeUser("t", tc.G)
	require.NoError(t, err)
	u2, err := kbtest.CreateAndSignupFakeUser("t", tc.G)
	require.NoError(t, err)

	repoName := "implicit repo foo bar"
	normalizedTLFName, err := kbfs.NormalizeNamesInTLF([]string{u1.Username, u2.Username}, nil, "")
	require.NoError(t, err)
	testFolder := keybase1.Folder{
		Name:       normalizedTLFName,
		Private:    true,
		FolderType: keybase1.FolderType_PRIVATE,
	}
	err = PutMetadata(context.TODO(), tc.G, keybase1.PutGitMetadataArg{
		Folder: testFolder,
		RepoID: keybase1.RepoID("abc123"),
		Metadata: keybase1.GitLocalMetadata{
			RepoName: keybase1.GitRepoName(repoName),
		},
	})
	require.NoError(t, err)

	assertStuffAboutRepo := func(t *testing.T, repo keybase1.GitRepoResult) {
		require.Equal(t, repoName, string(repo.LocalMetadata.RepoName))
		require.Equal(t, keybase1.FolderType_PRIVATE, repo.Folder.FolderType)
		require.Equal(t, true, repo.Folder.Private)
		require.Equal(t, kbtest.DefaultDeviceName, repo.ServerMetadata.LastModifyingDeviceName)
		require.Equal(t, "keybase://private/"+normalizedTLFName+"/"+repoName, repo.RepoUrl)
	}

	// Test fetching the implicit team repo with both Get and GetAll
	oneRepo, err := GetMetadata(context.Background(), tc.G, testFolder)
	require.NoError(t, err)
	require.Equal(t, 1, len(oneRepo))
	assertStuffAboutRepo(t, oneRepo[0])

	allRepos, err := GetAllMetadata(context.Background(), tc.G)
	require.NoError(t, err)
	require.Equal(t, 1, len(allRepos))
	assertStuffAboutRepo(t, allRepos[0])
}
