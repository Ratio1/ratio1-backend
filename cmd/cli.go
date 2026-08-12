package main

import (
	"time"

	"github.com/urfave/cli"
)

const (
	defaultLogsPath = "./"
	logPrefix       = "ratio1-api"
	logSaveFlag     = true
)

var (
	cliHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}}
{{if len .Authors}}
AUTHOR:
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .VisibleCommands}}
COMMANDS:
   {{range .VisibleCommands}}{{join .Names ", "}}{{"\t"}}{{.Usage}}
   {{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
VERSION:
   {{.Version}}
   {{end}}
`

	logLevel = cli.StringFlag{
		Name:  "log-level",
		Usage: "This flag specifies the log level. Options: | error | warn | info | debug | trace",
		Value: "info",
	}

	generalConfigFile = cli.StringFlag{
		Name:  "general-config",
		Usage: "The path for the general config",
		Value: "./config/",
	}

	workingDirectory = cli.StringFlag{
		Name:  "working-directory",
		Usage: "This flag specifies the directory where the proxy will store logs.",
		Value: "",
	}

	backgroundContextTimeout = 5 * time.Second
	migrationContextTimeout  = 15 * time.Minute
)
