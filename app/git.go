package app

// GitGetFile gets file from branch by project BackOfficeAppID, file relative path and branch path (reference path)
func (app *App) GitGetFile(projectID int, fileRelativePath string, branchPath string) ([]byte, error) {
	//file, _, err := app.Git.RepositoryFiles.GetRawFile(projectID, fileRelativePath, &gitlab.GetRawFileOptions{Ref: &branchPath})
	//return file, err
	return nil, nil
}
