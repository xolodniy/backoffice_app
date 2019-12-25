package model

import (
	"backoffice_app/common"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GuiaBolso/darwin"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Model is data tier of 3-layer architecture
type Model struct {
	db *gorm.DB
}

// New Model constructor
func New(db *gorm.DB) Model {
	return Model{
		db: db,
	}
}

// CheckMigrations validates database condition
func (m *Model) CheckMigrations() error {
	driver := darwin.NewGenericDriver(m.db.DB(), darwin.PostgresDialect{})
	d := darwin.New(driver, m.getMigrations(), nil)
	if err := d.Validate(); err != nil {
		return err
	}
	migrationInfo, err := d.Info()
	if err != nil {
		return err
	}
	for _, i := range migrationInfo {
		if i.Status == darwin.Applied {
			continue
		}
		return fmt.Errorf("found not applied migration: %s", i.Migration.Description)
	}
	return nil
}

// Migrate applies all migrations to connected database
func (m *Model) Migrate() {
	driver := darwin.NewGenericDriver(m.db.DB(), darwin.PostgresDialect{})
	d := darwin.New(driver, m.getMigrations(), nil)
	if err := d.Migrate(); err != nil {
		logrus.WithError(err).Error("can't migrate")
	}
}

// getMigrations provides migrations in darwin format
func (m *Model) getMigrations() []darwin.Migration {
	//migrationBox is used for embedding the migrations into the binary
	box := packr.NewBox("../etc/migrations")
	var migrations []darwin.Migration
	arr := box.List()
	sort.Strings(arr)
	for i, fileName := range arr {
		if !(strings.HasSuffix(fileName, ".sql") || strings.HasSuffix(fileName, ".SQL")) {
			logrus.Warnf("found file %s with unexpected type, skipping", fileName)
			continue
		}

		migration, err := box.FindString(fileName)
		if err != nil {
			logrus.WithError(err).Error("internal error of packr library")
		}
		migrations = append(migrations, darwin.Migration{
			Version:     float64(i + 1),
			Description: fileName,
			Script:      migration,
		})
	}
	return migrations
}

// CreateCommit creates new commit
func (m *Model) CreateCommit(commit Commit) error {
	if err := m.db.Create(&commit).Error; err != nil {
		logrus.WithError(err).WithField("commit", fmt.Sprintf("%+v", commit)).Error("can't create commit")
		return common.ErrInternal
	}
	return nil
}

// GetCommitsByType retrieves commits by type
func (m *Model) GetCommitsByType(commitsType string) ([]Commit, error) {
	var res []Commit
	if err := m.db.Find(&res).Where(Commit{Type: commitsType}).Error; err != nil {
		logrus.WithError(err).WithField("commitType", commitsType).Error("can't get commits")
		return nil, common.ErrInternal
	}
	return res, nil
}

// GetCommitByHash retrieves commit by hash
func (m *Model) GetCommitByHash(hash string) (Commit, error) {
	var res Commit
	if err := m.db.Where(Commit{Hash: hash}).First(&res).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return Commit{}, common.ErrNotFound
		}
		logrus.WithError(err).WithField("hash", hash).Error("can't get commit")
		return Commit{}, common.ErrInternal
	}
	return res, nil
}

// DeleteCommitsByType delete commits by type
func (m *Model) DeleteCommitsByType(commitsType string) error {
	var res []Commit
	if err := m.db.Where(Commit{Type: commitsType}).Delete(&res).Error; err != nil {
		logrus.WithError(err).WithField("commitType", commitsType).Error("can't delete commits")
		return common.ErrInternal
	}
	return nil
}

// CreateAfkTimer creates afk timer
func (m *Model) CreateAfkTimer(afkTimer AfkTimer) error {
	if err := m.db.Where(AfkTimer{UserId: afkTimer.UserId}).Assign(AfkTimer{Duration: afkTimer.Duration}).FirstOrCreate(&afkTimer).Error; err != nil {
		logrus.WithError(err).WithField("afkTimer", fmt.Sprintf("%+v", afkTimer)).Error("can't create afk timer")
		return common.ErrInternal
	}
	return nil
}

// GetAfkTimers retrieves afk timers
func (m *Model) GetAfkTimers() ([]AfkTimer, error) {
	var res []AfkTimer
	if err := m.db.Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get afk timers")
		return nil, common.ErrInternal
	}
	return res, nil
}

// DeleteAfkTimer deletes afk timer
func (m *Model) DeleteAfkTimer(userId string) error {
	var res []AfkTimer
	if err := m.db.Where(AfkTimer{UserId: userId}).Delete(&res).Error; err != nil {
		logrus.WithError(err).WithField("userId", userId).Error("can't delete afk timer by user id")
		return common.ErrInternal
	}
	return nil
}

// CreateVacation creates new vacation
func (m *Model) SaveVacation(vacation Vacation) error {
	if err := m.db.Where(Vacation{UserId: vacation.UserId}).Assign(Vacation{
		DateStart: vacation.DateStart,
		DateEnd:   vacation.DateEnd,
		Message:   vacation.Message,
	}).FirstOrCreate(&vacation).Error; err != nil {
		logrus.WithError(err).WithField("commit", fmt.Sprintf("%+v", vacation)).Error("can't create vacation")
		return common.ErrInternal
	}
	return nil
}

// GetVacation retrieves actual vacations by today
func (m *Model) GetActualVacations() ([]Vacation, error) {
	var res []Vacation
	if err := m.db.Where("date_start <= ? AND date_end >= ?", time.Now(), time.Now()).Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get vacations")
		return nil, common.ErrInternal
	}
	return res, nil
}

// GetVacation retrieves vacation by user id
func (m *Model) GetVacation(userId string) (Vacation, error) {
	var res Vacation
	if err := m.db.Find(&res).Where(Vacation{UserId: userId}).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return Vacation{}, common.ErrNotFound
		}
		logrus.WithError(err).WithField("userId", userId).Error("can't get vacation")
		return Vacation{}, common.ErrInternal
	}
	return res, nil
}

// DeleteVacation deletes vacation
func (m *Model) DeleteVacation(userId string) error {
	var res []Vacation
	if err := m.db.Where(Vacation{UserId: userId}).Delete(&res).Error; err != nil {
		logrus.WithError(err).WithField("userId", userId).Error("can't delete vacation by user id")
		return common.ErrInternal
	}
	return nil
}

// GetForgottenPullRequest retrieves old pull requests
func (m *Model) GetForgottenPullRequest() ([]ForgottenPullRequest, error) {
	var res []ForgottenPullRequest
	if err := m.db.Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get forgotten pull requests")
		return nil, common.ErrInternal
	}
	return res, nil
}

// CreateForgottenPullRequest creates old pull request
func (m *Model) CreateForgottenPullRequest(forgottenPullRequest ForgottenPullRequest) error {
	if err := m.db.Create(&forgottenPullRequest).Error; err != nil {
		logrus.WithError(err).WithField("forgottenPullRequest", fmt.Sprintf("%+v", forgottenPullRequest)).Error("can't create forgottenPullRequest")
		return common.ErrInternal
	}
	return nil
}

// DeleteForgottenPullRequest deletes forgotten pull request
func (m *Model) DeleteForgottenPullRequest(pullRequestID int, repoSlug string) error {
	if err := m.db.Where("pull_request_id = ? AND repo_slug = ?", pullRequestID, repoSlug).Delete(&ForgottenPullRequest{}).Error; err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"pullRequestID": pullRequestID, "repo_slug": repoSlug}).
			Error("can't delete forgotten pull request by pullRequestID and repo slug")
		return common.ErrInternal
	}
	return nil
}

// UpdateForgottenPullRequests updates forgotten pull request
func (m *Model) UpdateForgottenPullRequest(pullRequestID int, forgottenPullRequest ForgottenPullRequest) error {
	if err := m.db.Model(ForgottenPullRequest{}).Where(ForgottenPullRequest{PullRequestID: pullRequestID}).Update(forgottenPullRequest).Error; err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"pullRequestID":        pullRequestID,
			"forgottenPullRequest": fmt.Sprintf("%+v", forgottenPullRequest)}).
			Error("can't update forgottenPullRequest")
		return common.ErrInternal
	}
	return nil
}

// GetForgottenBranches retrieves old branches
func (m *Model) GetForgottenBranches() ([]ForgottenBranch, error) {
	var res []ForgottenBranch
	if err := m.db.Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get forgotten branches")
		return nil, common.ErrInternal
	}
	return res, nil
}

// CreateForgottenBranches creates old branch
func (m *Model) CreateForgottenBranches(forgottenBranch ForgottenBranch) error {
	if err := m.db.Create(&forgottenBranch).Error; err != nil {
		logrus.WithError(err).WithField("forgottenBranch", fmt.Sprintf("%+v", forgottenBranch)).Error("can't create forgottenBranch")
		return common.ErrInternal
	}
	return nil
}

// DeleteForgottenBranch deletes forgotten branch
func (m *Model) DeleteForgottenBranch(branchName, repoSlug string) error {
	if err := m.db.Where("name = ? AND repo_slug = ?", branchName, repoSlug).Delete(&ForgottenBranch{}).Error; err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"branchName": branchName, "repo_slug": repoSlug}).
			Error("can't delete forgotten branch by branch name and repo slug")
		return common.ErrInternal
	}
	return nil
}
