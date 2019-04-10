package model

import (
	"backoffice_app/common"
	"fmt"
	"sort"
	"strings"

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
		logrus.WithError(err).WithFields(logrus.Fields{
			"hash":       commit.Hash,
			"type":       commit.Type,
			"repository": commit.Repository,
			"path":       commit.Path,
			"message":    commit.Message,
		}).Error("can't create commit")
		return common.ErrInternal
	}
	return nil
}

// GetCommitsByType retrieves commits by type
func (m *Model) GetCommitsByType(commitsType string) ([]Commit, error) {
	var res []Commit
	if err := m.db.Find(&res).Where("type = ?", commitsType).Error; err != nil {
		logrus.WithError(err).Error("can't get commits")
		return nil, common.ErrInternal
	}
	return res, nil
}

// GetCommitByHash retrieves commits by hash and type
func (m *Model) GetCommitByHash(commitType, hash string) ([]Commit, error) {
	var res []Commit
	if err := m.db.Find(&res).Where("type = ? AND hash = ?", commitType, hash).Error; err != nil {
		logrus.WithError(err).Error("can't get commit")
		return nil, common.ErrInternal
	}
	return res, nil
}

// DeleteCommitsByType delete commits by type
func (m *Model) DeleteCommitsByType(commitsType string) error {
	var res []Commit
	if err := m.db.Where("type = ?", commitsType).Delete(&res).Error; err != nil {
		logrus.WithError(err).Error("can't delete commits")
		return common.ErrInternal
	}
	return nil
}

// CreateAfkTimer creates afk timer
func (m *Model) CreateAfkTimer(afkTimer AfkTimer) error {
	if err := m.db.Where(AfkTimer{UserId: afkTimer.UserId}).Assign(AfkTimer{Duration: afkTimer.Duration}).FirstOrCreate(&afkTimer).Error; err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"userId":    afkTimer.UserId,
			"duration":  afkTimer.Duration,
			"updatedAt": afkTimer.UpdatedAt,
		}).Error("can't create afk timer")
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
	if err := m.db.Where("user_id = ?", userId).Delete(&res).Error; err != nil {
		logrus.WithError(err).Error("can't delete afk timer")
		return common.ErrInternal
	}
	return nil
}
