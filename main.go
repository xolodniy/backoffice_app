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
	"backoffice_app/libs/task_manager"

	"github.com/jinzhu/now"
	"github.com/urfave/cli"
)

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

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
			wg := sync.WaitGroup{}
			tm := task_manager.New(&wg)

			tm.AddTask(cfg.WorkedTimeSendTime, func() {
				app.GetWorkersWorkedTimeAndSendToSlack(dateOfWorkdaysStart, dateOfWorkdaysEnd, cfg.Hubstaff.OrgsID)
			})

			/*tm.AddTask("@every 15m", func() {
				jiraAllIssues, _, err := cliApp.IssuesSearch(cfg.Jira.IssueSearchParams)
				if err != nil {
					panic(err)
				}
				log.Printf("jiraAllIssues quantity: %v\n", len(jiraAllIssues))
			})*/

			tm.Start()

			log.Println("Task scheduler started.")

			gracefulClosing(tm.Stop, &wg)
		}

		cliApp.Commands = []cli.Command{
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

					services.GetWorkersWorkedTimeAndSendToSlack(now.BeginningOfWeek(), now.EndOfWeek(), cfg.Hubstaff.OrgsID)

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

					services.GetWorkersWorkedTimeAndSendToSlack(now.BeginningOfDay(), now.EndOfDay(), cfg.Hubstaff.OrgsID)

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
