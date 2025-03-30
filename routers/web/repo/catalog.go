package repo

import (
	"net/http"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/modules/nessie"
)

// ListBranches returns a list of branches
func ListBranches(ctx *context.Context) {
	client := nessie.NewClient()
	branches, err := client.GetBranches(ctx.Repo.Repository.Name)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	
	// Transform to match expected frontend format
	results := make([]string, len(branches))
	for i, branch := range branches {
		results[i] = branch.Name
	}
	
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// ListTags returns a list of tags
func ListTags(ctx *context.Context) {
	client := nessie.NewClient()
	tags, err := client.GetTags(ctx.Repo.Repository.Name)
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	
	// Transform to match expected frontend format
	results := make([]string, len(tags))
	for i, tag := range tags {
		results[i] = tag.Name
	}
	
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"results": results,
	})
} 