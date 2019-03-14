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
	"backoffice_app/services/jira"

	"github.com/banzaicloud/logrus-runtime-formatter"
	"github.com/jinzhu/now"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	{
		cfg, err := config.GetConfig(true)
		if err != nil {
			panic(err)
		}

		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err != nil {
			panic("invalid logLevel \"" + cfg.LogLevel + " \" in cfg. available: ") //TODO + logrus.AllLevels()
		}
		logrus.SetLevel(level)

		formatter := runtime.Formatter{ChildFormatter: &logrus.TextFormatter{}}
		formatter.Line = true

		logrus.SetFormatter(&formatter)

		now.WeekStartDay = time.Monday

		cliApp := cli.NewApp()
		cliApp.Name = "Backoffice App"
		cliApp.Usage = "It's the best application for real time workers day and week progress."

		cliApp.Action = func(c *cli.Context) {
			application := app.New(cfg)

			go controller.New(*cfg).Start()
			log.Println("Requests listener started.")

			go application.FillCache()

			wg := sync.WaitGroup{}
			tm := initCronTasks(&wg, cfg, application)

			gracefulClosing(tm.Stop, &wg)
		}

		cliApp.Flags = []cli.Flag{
			cli.StringFlag{
				Name:  "channel, c",
				Value: "",
				Usage: "Channel for sending report, for example: -channel=#backoffice_app ",
			},
		}

		cliApp.Commands = []cli.Command{
			{
				Name:  "remove-all-slack-attachments",
				Usage: "Removes ABSOLUTELY ALL Slack attachments",
				Action: func(c *cli.Context) {
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
				Name:  "report-exceeded-estimate-now",
				Usage: "Reports exceeded estimate right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
					channel := c.String("channel")
					if channel == "" {
						logrus.Println("Empty channel flag!")
						return
					}
					application := app.New(cfg)
					application.ReportEmployeesWithExceededEstimateTime(channel)
				},
			},
			{
				Name:  "get-jira-issues-after-second-review-round-all",
				Usage: "Gets jira issues after second review round right now",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
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
				Name:  "get-slack-report-if-free-space-enging",
				Usage: "Gets report, if slack free space is empty",
				Flags: cliApp.Flags,
				Action: func(c *cli.Context) {
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
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportClarificationIssues()
				},
			},
			{
				Name:  "send-long-review-time-report-now",
				Usage: "Send long review time report right now",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.Report24HoursReviewIssues()
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

	err = tm.AddTask(cfg.Reports.EmployeesExceededEstimateTime.Schedule, func() {
		application.ReportEmployeesWithExceededEstimateTime(cfg.Reports.EmployeesExceededEstimateTime.Channel)
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
