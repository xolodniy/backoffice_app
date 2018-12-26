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

		switch cfg.LogLevel {
		case "debug":
			logrus.SetLevel(logrus.DebugLevel)
		case "info":
			logrus.SetLevel(logrus.InfoLevel)
		case "warn":
			logrus.SetLevel(logrus.WarnLevel)
		case "error":
			logrus.SetLevel(logrus.ErrorLevel)
		case "fatal":
			logrus.SetLevel(logrus.FatalLevel)
		case "panic":
			logrus.SetLevel(logrus.PanicLevel)
		default:
			panic("invalid logLevel \"" + cfg.LogLevel + " \" in cfg. available levels:\n" +
				"\t- debug\n" +
				"\t- info\n" +
				"\t- warn\n" +
				"\t- error\n" +
				"\t- fatal\n" +
				"\t- panic")
		}

		formatter := runtime.Formatter{ChildFormatter: &logrus.TextFormatter{}}
		formatter.Line = true

		logrus.SetFormatter(&formatter)

		now.WeekStartDay = time.Monday

		cliApp := cli.NewApp()
		cliApp.Name = "Backoffice App"
		cliApp.Usage = "It's the best application for real time workers day and week progress."

		cliApp.Action = func(c *cli.Context) {
			services, err := app.New(cfg)
			if err != nil {
				panic(err)
			}

			go controller.New(*cfg).Start()

			log.Println("Requests listener started.")

			wg := sync.WaitGroup{}
			tm := taskmanager.New(&wg)

			err = tm.AddTask(cfg.DailyReportCronTime, func() {
				services.GetWorkersWorkedTimeAndSendToSlack(
					"Daily work time report (auto)",
					now.BeginningOfDay().AddDate(0, 0, -1),
					now.EndOfDay().AddDate(0, 0, -1),
					cfg.Hubstaff.OrgsID)
			})
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.WeeklyReportCronTime, func() {
				services.GetWorkersWorkedTimeAndSendToSlack(
					"Weekly work time report (auto)",
					now.BeginningOfWeek().AddDate(0, 0, -1),
					now.EndOfWeek().AddDate(0, 0, -1),
					cfg.Hubstaff.OrgsID)
			})
			if err != nil {
				panic(err)
			}

			err = tm.AddTask(cfg.TaskTimeExceedionReportCronTime, func() {
				allIssues, _, err := services.IssuesSearch()
				if err != nil {
					panic(err)
				}
				var index = 1
				var msgBody = "Employees have exceeded tasks:\n"
				for _, issue := range allIssues {
					if issue.Fields != nil {
						if listRow := services.IssueTimeExceededNoTimeRange(issue, index); listRow != "" {
							msgBody += listRow
							index++
						}
					}
				}

				if err := services.Slack.SendStandardMessage(
					msgBody,
					services.Slack.Channel.ID,
					services.Slack.Channel.BotName,
				); err != nil {
					logrus.WithError(err).
						WithFields(logrus.Fields{
							"msgBody":        msgBody,
							"channelID":      services.Slack.Channel.ID,
							"channelBotName": services.Slack.Channel.BotName,
						}).
						Error("can't send Jira work time exceeding message to Slack.")
				}
			})
			if err != nil {
				panic(err)
			}

			tm.Start()

			log.Println("Task scheduler started.")

			gracefulClosing(tm.Stop, &wg)
		}

		cliApp.Commands = []cli.Command{

			{
				Name:  "get-jira-exceedions-now",
				Usage: "Gets jira exceedions right now",
				Action: func(c *cli.Context) {
					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					allIssues, _, err := services.IssuesSearch()
					if err != nil {
						panic(err)
					}
					var msgBody = "Employees have exceeded tasks:\n"
					var index = 1
					for _, issue := range allIssues {
						if issue.Fields != nil {
							if listRow := services.IssueTimeExceededNoTimeRange(issue, index); listRow != "" {
								msgBody += listRow
								index++
							}
						}
					}

					if err := services.Slack.SendStandardMessage(
						msgBody,
						services.Slack.Channel.ID,
						services.Slack.Channel.BotName,
					); err != nil {
						logrus.WithError(err).
							WithFields(logrus.Fields{
								"msgBody":        msgBody,
								"channelID":      services.Slack.Channel.ID,
								"channelBotName": services.Slack.Channel.BotName,
							}).
							Error("can't send Jira work time exceeding message to Slack.")
					}
				},
			},
			{
				Name:  "make-weekly-report-now",
				Usage: "Sends weekly report to slack channel",
				Action: func(c *cli.Context) {
					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(
						"Weekly work time report (manual)",
						now.BeginningOfWeek(),
						now.EndOfWeek(),
						cfg.Hubstaff.OrgsID)

				},
			},
			{
				Name:  "make-daily-report-now",
				Usage: "Sends daily report to slack channel",
				Action: func(c *cli.Context) {
					services, err := app.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(
						"Daily work time report (manual)",
						now.BeginningOfDay(),
						now.EndOfDay(), cfg.Hubstaff.OrgsID)

				},
			},
			{
				Name:  "obtain-hubstaff-token",
				Usage: "Obtains Hubstaff authorization token.",
				Action: func(c *cli.Context) {

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
