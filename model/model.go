package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"backoffice_app/common"

	"github.com/GuiaBolso/darwin"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Model is data tier of 3-layer architecture
type Model struct {
	db *gorm.DB

	// Used for tracing during building sql query.
	// Must be initialized separately for each query.
	logTrace logrus.Fields
}

// New Model constructor
func New(db *gorm.DB) Model {
	return Model{
		db: db,
	}
}

// StartTransaction initiate model layer as single transaction, you need to commit your changes at the end
func (m *Model) StartTransaction() (Model, error) {
	tx := Model{db: m.db.Begin()}
	if tx.db.Error != nil {
		logrus.WithError(tx.db.Error).Error("can't initiate database transaction")
		return Model{}, common.ErrInternal
	}
	return tx, nil
}

// CommitTransaction stories changes of transaction
func (m *Model) CommitTransaction() error {
	if err := m.db.Commit().Error; err != nil {
		logrus.WithError(err).Error("can't commit transaction")
		return common.ErrInternal
	}
	return nil
}

// RollBackTransaction skips changes from transaction exempts connection
func (m *Model) RollBackTransaction() error {
	if err := m.db.Rollback().Error; err != nil {
		logrus.WithError(err).Error("can't rollback transaction")
		return common.ErrInternal
	}
	return nil
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
			return Commit{}, common.ErrModelNotFound
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
	if err := m.db.Where(AfkTimer{UserID: afkTimer.UserID}).Assign(AfkTimer{Duration: afkTimer.Duration}).FirstOrCreate(&afkTimer).Error; err != nil {
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
func (m *Model) DeleteAfkTimer(userID string) error {
	var res []AfkTimer
	if err := m.db.Where(AfkTimer{UserID: userID}).Delete(&res).Error; err != nil {
		logrus.WithError(err).WithField("userID", userID).Error("can't delete afk timer by user id")
		return common.ErrInternal
	}
	return nil
}

// CreateVacation creates new vacation
func (m *Model) SaveVacation(vacation Vacation) error {
	if err := m.db.Where(Vacation{UserID: vacation.UserID}).Assign(Vacation{
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
func (m *Model) GetVacation(userID string) (Vacation, error) {
	var res Vacation
	if err := m.db.Find(&res).Where(Vacation{UserID: userID}).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return Vacation{}, common.ErrModelNotFound
		}
		logrus.WithError(err).WithField("userID", userID).Error("can't get vacation")
		return Vacation{}, common.ErrInternal
	}
	return res, nil
}

// DeleteVacation deletes vacation
func (m *Model) DeleteVacation(userId string) error {
	var res []Vacation
	if err := m.db.Where(Vacation{UserID: userId}).Delete(&res).Error; err != nil {
		logrus.WithError(err).WithField("userId", userId).Error("can't delete vacation by user id")
		return common.ErrInternal
	}
	return nil
}

// GetReminders retrieves reminders
func (m *Model) GetReminders() ([]Reminder, error) {
	var res []Reminder
	if err := m.db.Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get reminders")
		return nil, common.ErrInternal
	}
	return res, nil
}

// CreateReminder creates new reminder
func (m *Model) CreateReminder(reminder Reminder) {
	if err := m.db.Create(&reminder).Error; err != nil {
		logrus.WithError(err).WithField("reminder", fmt.Sprintf("%+v", reminder)).Error("can't create reminder")
	}
}

// DeleteReminder deletes reminder
func (m *Model) DeleteReminder(id int) error {
	if err := m.db.Where(Reminder{ID: id}).Delete(&Reminder{}).Error; err != nil {
		logrus.WithError(err).WithField("id", id).Error("can't delete reminder by id")
		return common.ErrInternal
	}
	return nil
}

// CreateRbAuth saves rbAuth
func (m *Model) CreateRbAuth(auth RbAuth) error {
	if err := m.db.Create(&auth).Error; err != nil {
		logrus.WithError(err).WithField("auth", fmt.Sprintf("%+v", auth)).Error("can't save rbAuth")
		return common.ErrInternal
	}
	return nil
}

// GetRbAuthByTgUserID retrieves RbAuth by user id
func (m *Model) GetRbAuthByTgUserID(TgUserID int64) (RbAuth, error) {
	var res RbAuth
	err := m.db.Take(&res, RbAuth{TgUserID: TgUserID}).Error
	if err == gorm.ErrRecordNotFound {
		return RbAuth{}, common.ErrModelNotFound
	}
	if err != nil {
		logrus.WithError(err).WithField("TgUserID", TgUserID).Error("can't get rb auth by user id")
		return RbAuth{}, common.ErrInternal
	}
	return res, nil
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

// CreateOnDutyUser creates user on duty
func (m *Model) CreateOnDutyUser(onDutyUser OnDutyUser) error {
	if err := m.db.Create(&onDutyUser).Error; err != nil {
		logrus.WithError(err).WithField("onDutyUser", fmt.Sprintf("%+v", onDutyUser)).Error("can't create onDutyUser")
		return common.ErrInternal
	}
	return nil
}

// DeleteOnDutyUsersByTeam deletes users on duty by team
func (m *Model) DeleteOnDutyUsersByTeam(team string) error {
	if err := m.db.Where(OnDutyUser{Team: team}).Delete(&[]OnDutyUser{}).Error; err != nil {
		logrus.WithError(err).WithField("team", team).Error("can't delete users on duty by team")
		return common.ErrInternal
	}
	return nil
}

// GetOnDutyUsersByTeam retrieves users on duty by team
func (m *Model) GetOnDutyUsersByTeam(team string) ([]OnDutyUser, error) {
	var res []OnDutyUser
	if err := m.db.Where(OnDutyUser{Team: team}).Find(&res).Error; err != nil {
		logrus.WithError(err).Error("can't get users on duty by team")
		return nil, common.ErrInternal
	}
	return res, nil
}

// Update is gorm interface func
func (m Model) Update(attrs ...interface{}) error {
	if err := m.db.Update(attrs...).Error; err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"updateAttrs":     fmt.Sprintf("%+v", attrs),
			"updateAttrsType": fmt.Sprintf("%T", attrs),
			"trace":           common.GetFrames(),
		}).Error("can't update object in database")
		return common.ErrInternal
	}
	return nil
}

// Model is gorm interface func
func (m Model) Model(value interface{}) *Model {
	trace := initLogTrace(m.logTrace)
	trace["modelValueType"] = fmt.Sprintf("%T", value)
	return &Model{db: m.db.Model(value), logTrace: trace}
}

// Unscoped is gorm interface func
func (m Model) Unscoped() Model {
	trace := initLogTrace(m.logTrace)
	trace["unscoped"] = true
	return Model{db: m.db.Unscoped(), logTrace: trace}
}

// Where is gorm interface func
func (m Model) Where(query interface{}, args ...interface{}) Model {
	trace := initLogTrace(m.logTrace)
	trace["whereQuery"] = fmt.Sprintf("%+v", query)
	trace["whereArgs"] = fmt.Sprintf("%+v", args)
	return Model{db: m.db.Where(query, args...), logTrace: trace}
}

// Save is gorm interface func
func (m *Model) Save(value interface{}) error {
	if err := m.db.Save(value).Error; err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"savedValue":     fmt.Sprintf("%+v", value),
			"savedValueType": fmt.Sprintf("%T", value),
			"trace":          common.GetFrames(),
		}).Error("can't save object in a database")
		return common.ErrInternal
	}
	return nil
}

// Create is gorm interface func
func (m *Model) Create(value interface{}) error {
	err := m.db.Create(value).Error
	if err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"createValue":       fmt.Sprintf("%+v", value),
			"createTypeOfValue": fmt.Sprintf("%T", value),
			"trace":             common.GetFrames(),
		}).Error("can't create value in database")
		return common.ErrInternal
	}
	return nil
}

func (m *Model) GetNamesOfProtectedBranchesAndPRs() ([]string, error) {
	var names []string
	if err := m.db.Model(Protected{}).Pluck("name", &names).Error; err != nil {
		logrus.WithError(err).Error("can't pluck names of protected branches and pull requests")
		return nil, common.ErrInternal
	}
	return names, nil
}

// Find is gorm interface func
func (m *Model) Find(out interface{}, where ...interface{}) error {
	err := m.db.Find(out, where...).Error
	if err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"findOutValue":       fmt.Sprintf("%+v", out),
			"findTypeOfOutValue": fmt.Sprintf("%T", out),
			"findWhereCondition": fmt.Sprintf("%+v", where),
			"trace":              common.GetFrames(),
		}).Error("can't find from the database")
		return common.ErrInternal
	}
	return nil
}

// First is gorm interface func
func (m *Model) First(out interface{}, where ...interface{}) error {
	err := m.db.First(out, where...).Error
	if gorm.IsRecordNotFoundError(err) {
		return common.ErrModelNotFound
	}
	if err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"firstOutValue":       fmt.Sprintf("%+v", out),
			"firstTypeOfOutValue": fmt.Sprintf("%T", out),
			"firstWhereCondition": fmt.Sprintf("%+v", where),
			"trace":               common.GetFrames(),
		}).Error("can't get first object from the database")
		return common.ErrInternal
	}
	return nil
}

// Last is gorm interface func
func (m *Model) Last(out interface{}, where ...interface{}) error {
	err := m.db.Last(out, where...).Error
	if gorm.IsRecordNotFoundError(err) {
		return common.ErrModelNotFound
	}
	if err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"lastOutValue":       fmt.Sprintf("%+v", out),
			"lastTypeOfOutValue": fmt.Sprintf("%T", out),
			"lastWhereCondition": fmt.Sprintf("%+v", where),
			"trace":              common.GetFrames(),
		}).Error("can't get last object from the database")
		return common.ErrInternal
	}
	return nil
}

// Delete is gorm interface func
func (m *Model) Delete(value interface{}, where ...interface{}) error {
	if err := m.db.Delete(value, where...).Error; err != nil {
		logrus.WithError(err).WithFields(m.logTrace).WithFields(logrus.Fields{
			"deleteValue": fmt.Sprintf("%+v", value),
			"deleteWhere": fmt.Sprintf("%+v", where),
			"trace":       common.GetFrames(),
		})
		return common.ErrInternal
	}
	return nil
}

func initLogTrace(trace logrus.Fields) logrus.Fields {
	if trace == nil {
		return make(logrus.Fields)
	}
	return trace
}
