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

			go application.FillCache()

			log.Println("Requests listener started.")

			wg := sync.WaitGroup{}
			tm := taskmanager.New(&wg)

			err = tm.AddTask(cfg.Cron.DailyWorkersWorkedTime, func() {
				application.MakeWorkersWorkedReportYesterday("auto")
			})
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.WeeklyWorkersWorkedTime, func() {
				application.MakeWorkersWorkedReportLastWeek("auto")
			})
			if err != nil {
				panic(err)
			}
			err = tm.AddTask(cfg.Cron.EmployeesExceededEstimateTime, application.ReportEmployeesWithExceededEstimateTime)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.EmployeesExceededTasks, application.ReportEmployeesHaveExceededTasks)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.ReportClosedSubtasks, application.ReportIsuuesWithClosedSubtasks)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.ReportAfterSecondReview, application.ReportIsuuesAfterSecondReview)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.ReportSlackSpaceEnding, application.ReportSlackEndingFreeSpace)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.ReportGitMigrations, application.ReportGitMigrations)
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.Cron.ReportSprintStatus, application.ReportSprintStatus)
			if err != nil {
				panic(err)
			}

			tm.Start()

			log.Println("Task scheduler started.")

			gracefulClosing(tm.Stop, &wg)
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
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportEmployeesHaveExceededTasks()
				},
			},
			{
				Name:  "get-jira-issues-with-closed-subtasks-now",
				Usage: "Gets jira issues with closed subtasks right now",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportIsuuesWithClosedSubtasks()
				},
			},
			{
				Name:  "report-exceeded-estimate-now",
				Usage: "Reports exceeded estimate right now",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportEmployeesWithExceededEstimateTime()
				},
			},
			{
				Name:  "get-jira-issues-after-second-review-round",
				Usage: "Gets jira issues after second review round right now",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportIsuuesWithClosedSubtasks()
				},
			},
			{
				Name:  "get-git-new-migrations",
				Usage: "Gets git new migrations right now",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportGitMigrations()
				},
			},
			{
				Name:  "get-slack-report-if-free-space-enging",
				Usage: "Gets report, if slack free space is empty",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportSlackEndingFreeSpace()
				},
			},
			{
				Name:  "get-slack-report-open-sprint-status",
				Usage: "Gets report about open sprint status",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportSprintStatus()
				},
			},
			{
				Name:  "make-weekly-report-now",
				Usage: "Sends weekly report to slack channel",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.MakeWorkersWorkedReportLastWeek("manual")
				},
			},
			{
				Name:  "make-daily-report-now",
				Usage: "Sends daily report to slack channel",
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.MakeWorkersWorkedReportYesterday("manual")
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
				Action: func(c *cli.Context) {
					application := app.New(cfg)
					application.ReportCurrentActivity()
				},
			},
		}

		if err := cliApp.Run(os.Args); err != nil {
			panic(err)
		}
	}

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
