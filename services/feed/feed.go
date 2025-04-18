// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"
	"fmt"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

func userFeedCacheKey(userID int64) string {
	return fmt.Sprintf("user_feed_%d", userID)
}

func GetFeedsForDashboard(ctx context.Context, opts activities_model.GetFeedsOptions) (activities_model.ActionList, int64, error) {
	opts.DontCount = opts.RequestedTeam == nil && opts.Date == ""
	results, cnt, err := activities_model.GetFeeds(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	if opts.DontCount {
		cnt, err = cache.GetInt64(userFeedCacheKey(opts.Actor.ID), func() (int64, error) {
			return activities_model.CountUserFeeds(ctx, opts.Actor.ID)
		})
	}
	return results, cnt, err
}

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts activities_model.GetFeedsOptions) (activities_model.ActionList, int64, error) {
	return activities_model.GetFeeds(ctx, opts)
}

// notifyWatchers creates batch of actions for every watcher.
// It could insert duplicate actions for a repository action, like this:
// * Original action: UserID=1 (the real actor), ActUserID=1
// * Organization action: UserID=100 (the repo's org), ActUserID=1
// * Watcher action: UserID=20 (a user who is watching a repo), ActUserID=1
func notifyWatchers(ctx context.Context, act *activities_model.Action, watchers []*repo_model.Watch, permCode, permIssue, permPR []bool) error {
	// Add feed for actioner.
	act.UserID = act.ActUserID
	if err := db.Insert(ctx, act); err != nil {
		return fmt.Errorf("insert new actioner: %w", err)
	}

	// Add feed for organization
	if act.Repo.Owner.IsOrganization() && act.ActUserID != act.Repo.Owner.ID {
		act.ID = 0
		act.UserID = act.Repo.Owner.ID
		if err := db.Insert(ctx, act); err != nil {
			return fmt.Errorf("insert new actioner: %w", err)
		}
	}

	for i, watcher := range watchers {
		if act.ActUserID == watcher.UserID {
			continue
		}
		act.ID = 0
		act.UserID = watcher.UserID
		act.Repo.Units = nil

		switch act.OpType {
		case activities_model.ActionCommitRepo, activities_model.ActionPushTag, activities_model.ActionDeleteTag, activities_model.ActionPublishRelease, activities_model.ActionDeleteBranch:
			if !permCode[i] {
				continue
			}
		case activities_model.ActionCreateIssue, activities_model.ActionCommentIssue, activities_model.ActionCloseIssue, activities_model.ActionReopenIssue:
			if !permIssue[i] {
				continue
			}
		case activities_model.ActionCreatePullRequest, activities_model.ActionCommentPull, activities_model.ActionMergePullRequest, activities_model.ActionClosePullRequest, activities_model.ActionReopenPullRequest, activities_model.ActionAutoMergePullRequest:
			if !permPR[i] {
				continue
			}
		}

		if err := db.Insert(ctx, act); err != nil {
			return fmt.Errorf("insert new action: %w", err)
		}

		total, err := activities_model.CountUserFeeds(ctx, act.UserID)
		if err != nil {
			return fmt.Errorf("count user feeds: %w", err)
		}

		_ = cache.GetCache().Put(userFeedCacheKey(act.UserID), fmt.Sprintf("%d", total), setting.CacheService.TTLSeconds())
	}

	return nil
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchers(ctx context.Context, acts ...*activities_model.Action) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if len(acts) == 0 {
			return nil
		}

		repoID := acts[0].RepoID
		if repoID == 0 {
			setting.PanicInDevOrTesting("action should belong to a repo")
			return nil
		}
		if err := acts[0].LoadRepo(ctx); err != nil {
			return err
		}
		repo := acts[0].Repo
		if err := repo.LoadOwner(ctx); err != nil {
			return err
		}

		actUserID := acts[0].ActUserID

		// Add feeds for user self and all watchers.
		watchers, err := repo_model.GetWatchers(ctx, repoID)
		if err != nil {
			return fmt.Errorf("get watchers: %w", err)
		}

		permCode := make([]bool, len(watchers))
		permIssue := make([]bool, len(watchers))
		permPR := make([]bool, len(watchers))
		for i, watcher := range watchers {
			user, err := user_model.GetUserByID(ctx, watcher.UserID)
			if err != nil {
				permCode[i] = false
				permIssue[i] = false
				permPR[i] = false
				continue
			}
			perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
			if err != nil {
				permCode[i] = false
				permIssue[i] = false
				permPR[i] = false
				continue
			}
			permCode[i] = perm.CanRead(unit.TypeCode)
			permIssue[i] = perm.CanRead(unit.TypeIssues)
			permPR[i] = perm.CanRead(unit.TypePullRequests)
		}

		for _, act := range acts {
			if act.RepoID != repoID {
				setting.PanicInDevOrTesting("action should belong to the same repo, expected[%d], got[%d] ", repoID, act.RepoID)
			}
			if act.ActUserID != actUserID {
				setting.PanicInDevOrTesting("action should have the same actor, expected[%d], got[%d] ", actUserID, act.ActUserID)
			}

			act.Repo = repo
			if err := notifyWatchers(ctx, act, watchers, permCode, permIssue, permPR); err != nil {
				return err
			}
		}
		return nil
	})
}
