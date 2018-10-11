package main

import (
	"backoffice_app/libs/task_manager"
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"backoffice_app/config"
	"backoffice_app/services"
)

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

func main() {
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
		services.GetWorkersWorkedTimeAndSendToSlack(dateOfWorkdaysStart, dateOfWorkdaysEnd, cfg.HubStaff.OrgsID)
	})

	tm.AddTask("@every 15m", func() {
		jiraAllIssues, _, err := services.Jira_IssuesSearch(cfg.Jira.IssueSearchParams)
		if err != nil {
			panic(err)
		}
		log.Printf("jiraAllIssues quantity: %v\n", len(jiraAllIssues))
	})

	tm.Start()

	log.Println("Task scheduler started.")

	gracefulClosing(tm.Stop, &wg)

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
