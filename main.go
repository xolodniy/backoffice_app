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
	"backoffice_app/libs/task_manager"

	"github.com/jinzhu/now"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

//var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
//var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

func main() {

	{
		now.WeekStartDay = time.Monday // Set Monday as first day, default is Sunday

		cliApp := cli.NewApp()
		cliApp.Name = "Backoffice App"
		cliApp.Usage = "It's the best application for real time workers day and week progress."

		cliApp.Action = func(c *cli.Context) {
			cfg, err := config.GetConfig(true)
			if err != nil {
				panic(err)
			}

			app, err := app.New(cfg)
			if err != nil {
				panic(err)
			}

			controller.New(*cfg).Start()

			wg := sync.WaitGroup{}
			tm := task_manager.New(&wg)

			tm.AddTask(cfg.DailyReportCronTime, func() {
				app.GetWorkersWorkedTimeAndSendToSlack(
					"Daily work time report",
					now.BeginningOfDay().AddDate(0, 0, -1),
					now.EndOfDay().AddDate(0, 0, -1),
					cfg.Hubstaff.OrgsID)
			})

			tm.AddTask(cfg.WeeklyReportCronTime, func() {
				app.GetWorkersWorkedTimeAndSendToSlack(
					"Weekly work time report",
					now.BeginningOfWeek().AddDate(0, 0, -1),
					now.EndOfWeek().AddDate(0, 0, -1),
					cfg.Hubstaff.OrgsID)
			})

			tm.AddTask(cfg.TaskTimeExceedionReportCronTime, func() {
				allIssues, _, err := app.IssuesSearch()
				if err != nil {
					panic(err)
				}
				var msgBody = "Employees have exceeded tasks:\n"
				for index, issue := range allIssues {
					if issue.Fields.TimeSpent > issue.Fields.TimeOriginalEstimate {
						ts, err := app.SecondsToClockTime(issue.Fields.TimeSpent)
						if err != nil {
							logrus.Errorf("time spent conversion: regexp error: %v", err)
							continue
						}

						te, err := app.SecondsToClockTime(issue.Fields.TimeOriginalEstimate)
						if err != nil {
							logrus.Errorf("time spent conversion: regexp error: %v", err)
							continue
						}

						msgBody += fmt.Sprintf("%d. %s - %s: %v из %v\n",
							index, issue.Key, issue.Fields.Summary, ts, te,
						)

					}

				}

				app.Slack.SendStandardMessage(
					msgBody,
					app.Slack.Channel.ID,
					app.Slack.Channel.BotName,
				)
			})

			tm.Start()

			log.Println("Task scheduler started.")

			gracefulClosing(tm.Stop, &wg)
		}

		cliApp.Commands = []cli.Command{

			{
				Name:  "get-jira-exceedions-now",
				Usage: "Gets jira exceedions right now",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig(true)
					if err != nil {
						panic(err)
					}

					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					allIssues, _, err := services.IssuesSearch()
					if err != nil {
						panic(err)
					}
					var msgBody = "Employees have exceeded tasks:\n"
					for index, issue := range allIssues {
						if issue.Fields.TimeSpent > issue.Fields.TimeOriginalEstimate {
							ts, err := services.SecondsToClockTime(issue.Fields.TimeSpent)
							te, err := services.SecondsToClockTime(issue.Fields.TimeOriginalEstimate)
							if err != nil {
								logrus.Errorf("time conversion: regexp error: %v", err)
								continue
							}

							msgBody += fmt.Sprintf("%d. %s - %s: %v из %v\n",
								index, issue.Key, issue.Fields.Summary, ts, te,
							)

						}

					}

					services.Slack.SendStandardMessage(
						msgBody,
						services.Slack.Channel.ID,
						services.Slack.Channel.BotName,
					)
				},
			},
			{
				Name:  "make-weekly-report-now",
				Usage: "Sends weekly report to slack channel",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig(true)
					if err != nil {
						panic(err)
					}

					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(
						"Weekly work time report",
						now.BeginningOfWeek(),
						now.EndOfWeek(), cfg.Hubstaff.OrgsID)

				},
			},
			{
				Name:  "make-daily-report-now",
				Usage: "Sends daily report to slack channel",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig(true)
					if err != nil {
						panic(err)
					}

					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(
						"Daily work time report",
						now.BeginningOfDay(),
						now.EndOfDay(), cfg.Hubstaff.OrgsID)

				},
			},
			{
				Name:  "obtain-hubstaff-token",
				Usage: "Obtains Hubstaff authorization token.",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig(true)
					if err != nil {
						panic(err)
					}

					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}
					authToken, err := services.Hubstaff.ObtainAuthToken(cfg.Hubstaff.Auth)
					if err != nil {
						panic(err)
					}
					fmt.Printf("Hubstaff auth token is:\n%s\n", authToken)
				},
			},
		}
		cliApp.Run(os.Args)
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
