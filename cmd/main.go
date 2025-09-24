package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/templates"
	"github.com/robfig/cron/v3"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	cli.AppHelpTemplate = cliHelpTemplate
	app.Name = "ratio1-api"
	app.Flags = []cli.Flag{
		generalConfigFile,
		workingDirectory,
	}
	app.Authors = []cli.Author{
		{
			Name:  "The <Placeholder> Team",
			Email: "contact@<placeholder>.com",
		},
	}
	app.Action = startApi

	err := app.Run(os.Args)
	if err != nil {
		err = errors.New("error while running application: " + err.Error())
		panic(err)
	}
}

func startApi(ctx *cli.Context) error {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		err = errors.New("error while retrieving node address: " + err.Error())
		return err
	}

	generalConfigPath := ctx.GlobalString(generalConfigFile.Name)
	network := os.Getenv("EE_EVM_NET")
	if network == "" {
		err = errors.New("EE_EVM_NET environment variable not set, cannot load config")
		return err
	}

	cfg, err := config.LoadConfig(generalConfigPath + "config." + network + ".json")
	if err != nil {
		err = errors.New("error while loading configs: " + err.Error())
		return err
	}

	config.Config = *cfg

	storage.Connect()
	templates.LoadAndCacheTemplates()

	if !config.Config.Api.DevTesting {
		buyLicenseInvoiceNodeTiming, found := config.Config.GetBuyLicenseInvoiceCronJobTiming(nodeAddress)
		if found {
			c := cron.New()
			_, err = c.AddFunc(buyLicenseInvoiceNodeTiming, service.ElaborateInvoices)
			if err != nil {
				return errors.New("error while starting cronjob: " + err.Error())
			}
			c.Start()
		}

		dailyNodeTiming, found := config.Config.GetDailyCronJobTiming(nodeAddress)
		if found {
			c := cron.New()
			_, err = c.AddFunc(dailyNodeTiming, service.DailyGetStats)
			if err != nil {
				return errors.New("error while starting daily cronjob: " + err.Error())
			}
			c.Start()
		}

		monthlyNodeTiming, found := config.Config.GetMonthlyCronJobTiming(nodeAddress)
		if found {
			c := cron.New()
			_, err = c.AddFunc(monthlyNodeTiming, service.MonthlyPoaiInvoiceReport)
			if err != nil {
				return errors.New("error while starting daily cronjob: " + err.Error())
			}
			c.Start()
		}
	}

	api, err := proxy.NewWebServer()
	if err != nil {
		return errors.New("error while starting new web server: " + err.Error())
	}
	server := api.Run()

	waitForGracefulShutdown(server)

	return nil
}

func waitForGracefulShutdown(server *http.Server) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, os.Kill)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), backgroundContextTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		panic(err)
	}
	_ = server.Close()
}
