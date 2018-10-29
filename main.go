package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"backoffice_app/config"
	"backoffice_app/libs/task_manager"
	"backoffice_app/services"

	"github.com/jinzhu/now"
	"github.com/urfave/cli"
)

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

func main() {
	{
		now.WeekStartDay = time.Monday // Set Monday as first day, default is Sunday

		app := cli.NewApp()
		app.Name = "Backoffice App"
		app.Usage = "It's the best application for real time workers day and week progress."

		app.Action = func(c *cli.Context) {
			cfg, err := config.GetConfig()
			if err != nil {
				panic(err)
			}
			services, err := services.New(cfg)
			if err != nil {
				panic(err)
			}

			wg := sync.WaitGroup{}
			tm := task_manager.New(&wg)

			tm.AddTask(cfg.WorkedTimeSendTime, func() {
				services.GetWorkersWorkedTimeAndSendToSlack(dateOfWorkdaysStart, dateOfWorkdaysEnd, cfg.Hubstaff.OrgsID)
			})

			/*tm.AddTask("@every 15m", func() {
				jiraAllIssues, _, err := services.Jira_IssuesSearch(cfg.Jira.IssueSearchParams)
				if err != nil {
					panic(err)
				}
				log.Printf("jiraAllIssues quantity: %v\n", len(jiraAllIssues))
			})*/

			tm.Start()

			log.Println("Task scheduler started.")

			gracefulClosing(tm.Stop, &wg)
		}

		app.Commands = []cli.Command{
			{
				Name:  "make-weekly-report-now",
				Usage: "Sends weekly report to slack channel",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig()
					if err != nil {
						panic(err)
					}

					services, err := services.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(now.BeginningOfWeek(), now.EndOfWeek(), cfg.Hubstaff.OrgsID)

				},
			},
		}

		app.Commands = []cli.Command{
			{
				Name:  "make-daily-report-now",
				Usage: "Sends daily report to slack channel",
				Action: func(c *cli.Context) {
					cfg, err := config.GetConfig()
					if err != nil {
						panic(err)
					}

					services, err := services.New(cfg)
					if err != nil {
						panic(err)
					}

					services.GetWorkersWorkedTimeAndSendToSlack(now.BeginningOfDay(), now.EndOfDay(), cfg.Hubstaff.OrgsID)

				},
			},
		}
		app.Run(os.Args)
	}

}

func gracefulClosing(cancel context.CancelFunc, servicesWg *sync.WaitGroup) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("stopping services... (Double enter Ctrl + C to force close)")
	cancel()

	quit := make(chan struct{})
	go func() {
		<-sig
		<-sig
		log.Println("services unsafe stopped")
		<-quit
	}()

	go func() {
		servicesWg.Wait()
		log.Println("services gracefully stopped")
		<-quit
	}()

	quit <- struct{}{}
	close(quit)
}
