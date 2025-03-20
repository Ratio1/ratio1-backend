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
	nodes, err := config.LoadNodes(generalConfigPath + "nodes.json")
	if err != nil {
		err = errors.New("error while loading nodes: " + err.Error())
		return err
	}

	if value, exist := nodes[nodeAddress]; exist {
		cfg, err := config.LoadConfig(generalConfigPath + "config" + value + ".json")
		if err != nil {
			err = errors.New("error while loading configs: " + err.Error())
			return err
		}

		config.Config = *cfg
	} else {
		return errors.New("the node is not in the whitelist")
	}

	storage.Connect()
	templates.LoadAndCacheTemplates()

	if !config.Config.Api.DevTesting {
		nodeTiming, found := config.Config.GetCronJobTiming(nodeAddress)
		if found {
			c := cron.New()
			_, err = c.AddFunc(nodeTiming, service.ElaborateInvoices)
			if err != nil {
				return errors.New("error while starting cronjob: " + err.Error())
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
