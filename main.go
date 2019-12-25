package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"backoffice_app/app"
	"backoffice_app/config"
	"backoffice_app/controller"
	"backoffice_app/libs/taskmanager"
	"backoffice_app/model"
	"backoffice_app/services/jira"

	runtime "github.com/banzaicloud/logrus-runtime-formatter"
	"github.com/jinzhu/gorm"
	"github.com/jinzhu/now"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	{

		cliApp := cli.NewApp()
		cliApp.Name = "Backoffice App"
		cliApp.Usage = "It's the best application for real time workers day and week progress."

		cliApp.Flags = []cli.Flag{
			cli.StringFlag{
				Name:  "config",
				Value: "/etc/backoffice_app/config.yml",
				Usage: "optional config path",
			},
			cli.StringFlag{
				Name:  "channel, c",
				Value: "",
				Usage: "Channel for sending report, for example: -channel=#backoffice_app ",
			},
		}

		cliApp.Action = func(c *cli.Context) {
			cfg := config.GetConfig(true, c.String("config"))

			level, err := logrus.ParseLevel(cfg.LogLevel)
			if err != nil {
				panic(fmt.Sprintf("invalid logLevel \"%s\" in cfg. available: %s", cfg.LogLevel, logrus.AllLevels))
			}
			logrus.SetLevel(level)

			formatter := runtime.Formatter{ChildFormatter: &logrus.TextFormatter{}}
			formatter.Line = true
			logrus.SetFormatter(&formatter)

			now.WeekStartDay = time.Monday
			application := app.New(cfg)

			go controller.New(*cfg, application).Start()
			log.Println("Requests listener started.")

			go application.CheckAfkTimers()
			go application.FillCache()

			wg := sync.WaitGroup{}
			tm := initCronTasks(&wg, cfg, application)

			gracefulClosing(tm.Stop, &wg)
		}

		cliApp.Commands = []cli.Command{
			{
				Name:  "migrate",
				Usage: "update migrations to the latest stage",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))

					db, err := gorm.Open("postgres", cfg.Database.ConnURL())
					if err != nil {
						logrus.WithError(err).Fatal("can't open connection with a database")
					}
					if err := db.DB().Ping(); err != nil {
						logrus.WithError(err).Fatal("can't ping connection with a database")
					}
					m := model.New(db)
					m.Migrate()
				},
			},
			{
				Name:  "remove-all-slack-attachments",
				Usage: "Removes ABSOLUTELY ALL Slack attachments",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					application := app.New(cfg)
					files, err := application.Slack.Files()
					if len(files) == 0 {
						// We finished.
						return
					}
					if err != nil {
						panic(err)
					}
					for _, f := range files {
						if err := application.Slack.DeleteFile(f.ID); err != nil {
							panic(err)
						}
						logrus.Info("deleted file " + f.ID)
					}
				},
			},
			{
				Name:  "get-jira-exceedions-now",
				Usage: "Gets jira exceedions right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportEmployeesHaveExceededTasks(channel)
				},
			},
			{
				Name:  "get-jira-issues-with-closed-subtasks-now",
				Usage: "Gets jira issues with closed subtasks right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportIsuuesWithClosedSubtasks(channel)
				},
			},
			{
				Name:  "get-jira-issues-after-second-review-round-all",
				Usage: "Gets jira issues after second review round right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportIssuesAfterSecondReview(channel)
				},
			},
			{
				Name:  "get-jira-issues-after-second-review-round-be",
				Usage: "Gets jira backend issues after second review round right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportIssuesAfterSecondReview(channel, jira.TypeBETask, jira.TypeBESubTask)
				},
			},
			{
				Name:  "get-jira-issues-after-second-review-round-fe",
				Usage: "Gets jira frontend issues after second review round right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportIssuesAfterSecondReview(channel, jira.TypeFETask, jira.TypeFESubTask)
				},
			},
			{
				Name:  "get-git-new-migrations",
				Usage: "Gets git new migrations right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportGitMigrations(channel)
				},
			},
			{
				Name:  "get-git-new-ansible-changes",
				Usage: "Gets git new ansible changes right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportGitAnsibleChanges(channel)
				},
			},
			{
				Name:  "get-slack-report-if-free-space-enging",
				Usage: "Gets report, if slack free space is empty",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportSlackEndingFreeSpace(channel)
				},
			},
			{
				Name:  "get-slack-report-open-sprint-status",
				Usage: "Gets report about open sprint status",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportSprintStatus(channel)
				},
			},
			{
				Name:  "make-weekly-report-now",
				Usage: "Sends weekly report to slack channel",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.MakeWorkersWorkedReportLastWeek("manual", channel)
				},
			},
			{
				Name:  "make-daily-report-now",
				Usage: "Sends daily report to slack channel",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.MakeWorkersWorkedReportYesterday("manual", channel)
				},
			},
			{
				Name:  "obtain-hubstaff-token",
				Usage: "Obtains Hubstaff authorization token.",
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					application := app.New(cfg)

					authToken, err := application.Hubstaff.ObtainAuthToken(cfg.Hubstaff.Auth)
					if err != nil {
						panic(err)
					}
					fmt.Printf("Hubstaff auth token is:\n%s\n", authToken)
				},
			},
			{
				Name:  "send-last-activity-report-now",
				Usage: "Send last activity report right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportCurrentActivity(channel)
				},
			},
			{
				Name:  "send-clarification-report-now",
				Usage: "Send clarification issues report right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					application := app.New(cfg)
					application.ReportClarificationIssues()
				},
			},
			{
				Name:  "send-long-review-time-report-now",
				Usage: "Send long review time report right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					application := app.New(cfg)
					application.Report24HoursReviewIssues()
				},
			},
			{
				Name:  "make-report-workers-less-worked-now",
				Usage: "Sends daily report about user that worked less then 6h to slack channel",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.MakeWorkersLessWorkedReportYesterday(channel)
				},
			},
			{
				Name:  "make-report-overworked-issues-now",
				Usage: "Sends report about overworked issues during the last week",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportOverworkedIssues(channel)
				},
			},
			{
				Name:  "get-jira-epics-with-closed-issues-now",
				Usage: "Gets jira epics with closed issues right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportEpicsWithClosedIssues(channel)
				},
			},
			{
				Name:  "get-jira-blocked-issues-with-low-priority",
				Usage: "Gets jira low-priority blockers for current sprint issues",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportIssuesLockedByLowPriority(channel)
				},
			},
			{
				Name:  "get-low-priority-issues-started",
				Usage: "Gets jira low priority issues started by developer",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportLowPriorityIssuesStarted(channel)
				},
			},
			{
				Name:  "check-need-reply-messages",
				Usage: "Send message about need reply for mention",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					application := app.New(cfg)
					application.CheckNeedReplyMessages()
				},
			},
			{
				Name:  "check-old-prs",
				Usage: "Check old pull requests in bitbucket",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.CheckForgottenGitPullRequests(channel)
				},
			},
			{
				Name:  "check-old-branches",
				Usage: "Check old branches in bitbucket",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					cfg := config.GetConfig(true, c.String("config"))
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.CheckForgottenGitBranches(channel)
				},
			},
		}

		if err := cliApp.Run(os.Args); err != nil {
			panic(err)
		}
	}

}

func initCronTasks(wg *sync.WaitGroup, cfg *config.Main, application *app.App) *taskmanager.TaskManager {
	tm := taskmanager.New(wg)

	err := tm.AddTask(cfg.Reports.DailyWorkersWorkedTime.Schedule, func() {
		application.MakeWorkersWorkedReportYesterday("auto", cfg.Reports.DailyWorkersWorkedTime.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.WeeklyWorkersWorkedTime.Schedule, func() {
		application.MakeWorkersWorkedReportLastWeek("auto", cfg.Reports.WeeklyWorkersWorkedTime.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.EmployeesExceededTasks.Schedule, func() {
		application.ReportEmployeesHaveExceededTasks(cfg.Reports.EmployeesExceededTasks.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportClosedSubtasks.Schedule, func() {
		application.ReportIsuuesWithClosedSubtasks(cfg.Reports.ReportClosedSubtasks.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportAfterSecondReviewAll.Schedule, func() {
		application.ReportIssuesAfterSecondReview(cfg.Reports.ReportAfterSecondReviewAll.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportAfterSecondReviewBE.Schedule, func() {
		application.ReportIssuesAfterSecondReview(cfg.Reports.ReportAfterSecondReviewBE.Channel, jira.TypeBETask, jira.TypeBESubTask)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportAfterSecondReviewFE.Schedule, func() {
		application.ReportIssuesAfterSecondReview(cfg.Reports.ReportAfterSecondReviewFE.Channel, jira.TypeFETask, jira.TypeFESubTask)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportSlackSpaceEnding.Schedule, func() {
		application.ReportSlackEndingFreeSpace(cfg.Reports.ReportSlackSpaceEnding.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportGitMigrations.Schedule, func() {
		application.ReportGitMigrations(cfg.Reports.ReportGitMigrations.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportSprintStatus.Schedule, func() {
		application.ReportSprintStatus(cfg.Reports.ReportSprintStatus.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportClarificationIssues.Schedule, func() {
		application.ReportClarificationIssues()
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.Report24HoursReviewIssues.Schedule, func() {
		application.Report24HoursReviewIssues()
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportGitAnsibleChanges.Schedule, func() {
		application.ReportGitAnsibleChanges(cfg.Reports.ReportGitAnsibleChanges.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.DailyWorkersLessWorkedMessage.Schedule, func() {
		application.MakeWorkersLessWorkedReportYesterday(cfg.Reports.DailyWorkersLessWorkedMessage.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.WeeklyReportOverworkedIssues.Schedule, func() {
		application.ReportOverworkedIssues(cfg.Reports.WeeklyReportOverworkedIssues.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportEpicClosedIssues.Schedule, func() {
		application.ReportEpicsWithClosedIssues(cfg.Reports.ReportEpicClosedIssues.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportLowPriorityIssuesStarted.Schedule, func() {
		application.ReportLowPriorityIssuesStarted(cfg.Reports.ReportLowPriorityIssuesStarted.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.CheckNeedReplyMessages.Schedule, func() {
		application.CheckNeedReplyMessages()
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.CheckLowerPriorityBlockers.Schedule, func() {
		application.ReportIssuesLockedByLowPriority(cfg.Reports.CheckLowerPriorityBlockers.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportForgottenPRs.Schedule, func() {
		application.CheckForgottenGitPullRequests(cfg.Reports.ReportForgottenPRs.Channel)
	})
	if err != nil {
		panic(err)
	}

	err = tm.AddTask(cfg.Reports.ReportForgottenBranches.Schedule, func() {
		application.CheckForgottenGitBranches(cfg.Reports.ReportForgottenBranches.Channel)
	})
	if err != nil {
		panic(err)
	}

	tm.Start()
	log.Println("Task scheduler started.")

	return tm
}

func gracefulClosing(cancel context.CancelFunc, servicesWg *sync.WaitGroup) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("stopping app... (Double enter Ctrl + C to force close)")
	cancel()

	quit := make(chan struct{})
	go func() {
		<-sig
		<-sig
		log.Println("app unsafe stopped")
		<-quit
	}()

	go func() {
		servicesWg.Wait()
		log.Println("app gracefully stopped")
		<-quit
	}()

	quit <- struct{}{}
	close(quit)
}
